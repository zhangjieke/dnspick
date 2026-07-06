package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
	"golang.org/x/term"

	"github.com/zhangjieke/dnspick/internal/buildinfo"
	"github.com/zhangjieke/dnspick/internal/config"
	"github.com/zhangjieke/dnspick/internal/console"
	"github.com/zhangjieke/dnspick/internal/dnsbench"
	"github.com/zhangjieke/dnspick/internal/i18n"
	"github.com/zhangjieke/dnspick/internal/ui"
	"github.com/zhangjieke/dnspick/internal/updater"
)

var (
	domainsStr       string
	serversStr       string
	queriesPerDomain int
	queryTimeout     time.Duration
	maxConcurrency   int
	noSystemDNS      bool
	langFlag         string
	jsonOutput       bool
)

var (
	ensureDomainList  = config.EnsureDomainList
	ensureServerList  = config.EnsureServerList
	loadDomainEntries = config.LoadDomainEntries
	loadServerEntries = config.LoadServerEntries
	detectSystemDNS   = dnsbench.DetectSystemDNS
)

var rootCmd = &cobra.Command{
	Use:           "dnspick",
	Version:       buildinfo.Version,
	RunE:          runBenchmark,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildinfo.String())
	},
}

var updateCmd = &cobra.Command{
	Use:  "update",
	RunE: runUpdate,
}

// setup wires up localized help text, flags and subcommands. It must run after
// the active language has been selected so that --help reflects --lang/$LANG.
func setup() {
	m := i18n.L()

	// Cobra's Windows "mousetrap" otherwise intercepts a double-click launch,
	// prints "This is a command line tool..." and exits before the command
	// runs. dnspick is meant to be usable by double-clicking, and the console
	// is kept open afterwards by console.PauseOnExit, so disable the mousetrap.
	cobra.MousetrapHelpText = ""

	rootCmd.Short = m.CmdRootShort
	rootCmd.Long = m.CmdRootLong
	versionCmd.Short = m.CmdVersionShort
	updateCmd.Short = m.CmdUpdateShort

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	flags := rootCmd.PersistentFlags()
	flags.StringVarP(&domainsStr, "domains", "d", "", m.FlagDomains)
	flags.StringVarP(&serversStr, "servers", "s", "", m.FlagServers)
	flags.IntVarP(&queriesPerDomain, "queries", "q", 3, m.FlagQueries)
	flags.DurationVarP(&queryTimeout, "timeout", "t", 2*time.Second, m.FlagTimeout)
	flags.IntVarP(&maxConcurrency, "concurrency", "c", 16, m.FlagConcurrency)
	flags.BoolVar(&noSystemDNS, "no-system-dns", false, m.FlagNoSystemDNS)
	flags.StringVar(&langFlag, "lang", "", m.FlagLang)
	flags.BoolVar(&jsonOutput, "json", false, m.FlagJSON)

	rootCmd.AddCommand(versionCmd, updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), updater.DefaultTimeout)
	defer cancel()

	m := i18n.L()
	fmt.Printf(m.UpdateChecking, buildinfo.Version)
	latest, updated, err := updater.Update(ctx, buildinfo.Version)
	if err != nil {
		return fmt.Errorf("%s %w", m.UpdateFailed, err)
	}
	if !updated {
		fmt.Printf(m.UpdateUpToDate, latest)
		return nil
	}
	fmt.Printf(m.UpdateDone, latest)
	return nil
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	m := i18n.L()

	domains, err := benchmarkDomains(cmd)
	if err != nil {
		return err
	}
	if len(domains) == 0 {
		return fmt.Errorf("%s", m.ErrNoDomains)
	}

	servers, err := benchmarkServers(cmd, m.SystemDNSName, m.SystemDNSNameN)
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return fmt.Errorf("%s", m.ErrNoServers)
	}

	opts := dnsbench.Options{
		Servers:     servers,
		Domains:     domains,
		Queries:     queriesPerDomain,
		Timeout:     queryTimeout,
		Concurrency: maxConcurrency,
	}

	// JSON mode: stdout carries only the JSON document, status goes to stderr,
	// and the live progress UI is skipped so the output stays pipe-friendly.
	if jsonOutput {
		fmt.Fprintf(os.Stderr, m.BenchStarting, len(servers), len(domains))
		results := dnsbench.Run(opts, nil)
		return ui.WriteJSON(os.Stdout, results, queriesPerDomain, len(domains))
	}

	// Kick off a non-blocking check for a newer release; it runs concurrently
	// with the benchmark and the notice (if any) is printed at the end.
	updateCh := startUpdateCheck()

	fmt.Printf(m.BenchStarting, len(servers), len(domains))

	tracker := ui.NewStatusTracker(domains, len(servers), queriesPerDomain)
	tracker.Start()
	results := dnsbench.Run(opts, tracker.Progress)
	tracker.Stop()

	fmt.Println(m.ResultsHeader)
	ui.PrintResultsTable(results)

	fmt.Println(m.RecommendHeader)
	ui.PrintRecommendations(results)

	autoUpdate(updateCh)
	return nil
}

func benchmarkDomains(cmd *cobra.Command) ([]dnsbench.Domain, error) {
	if cmd.Flags().Changed("domains") {
		return dnsbench.ParseDomains(domainsStr), nil
	}

	if err := ensureDomainList(defaultDomainEntries()); err != nil {
		return nil, fmt.Errorf("ensure domain.list: %w", err)
	}
	entries, exists, err := loadDomainEntries()
	if err != nil {
		return nil, fmt.Errorf("load domain.list: %w", err)
	}
	if exists {
		return dnsbench.ParseDomainEntries(slices.Values(entries)), nil
	}
	return slices.Clone(dnsbench.DefaultDomains), nil
}

func benchmarkServers(cmd *cobra.Command, systemName, systemNameFmt string) ([]dnsbench.Server, error) {
	var servers []dnsbench.Server
	if cmd.Flags().Changed("servers") {
		servers = dnsbench.ParseServers(serversStr)
		if len(servers) == 0 {
			return servers, nil
		}
	} else {
		if err := ensureServerList(defaultServerEntries()); err != nil {
			return nil, fmt.Errorf("ensure server.list: %w", err)
		}
		entries, exists, err := loadServerEntries()
		if err != nil {
			return nil, fmt.Errorf("load server.list: %w", err)
		}
		if exists {
			servers = dnsbench.ParseServerEntries(slices.Values(entries))
		} else {
			servers = slices.Clone(dnsbench.DefaultServers)
		}
	}

	if !noSystemDNS {
		sys := detectSystemDNS(systemName, systemNameFmt)
		servers = dnsbench.MergeServers(servers, sys)
	}
	return servers, nil
}

func defaultDomainEntries() []string {
	entries := make([]string, 0, len(dnsbench.DefaultDomains))
	for _, d := range dnsbench.DefaultDomains {
		entries = append(entries, d.Name)
	}
	return entries
}

func defaultServerEntries() []string {
	entries := make([]string, 0, len(dnsbench.DefaultServers))
	for _, s := range dnsbench.DefaultServers {
		entries = append(entries, serverConfigEntry(s))
	}
	return entries
}

func serverConfigEntry(s dnsbench.Server) string {
	switch s.Protocol {
	case dnsbench.DOT:
		return "tls://" + s.Address
	case dnsbench.DOH3:
		return "h3://" + strings.TrimPrefix(s.Address, "https://")
	default:
		return s.Address
	}
}

// updateCheckTimeout bounds the background "is there a newer release?" check so a
// slow or unreachable network never holds anything up for long.
const updateCheckTimeout = 3 * time.Second

// updateNoticeGrace is how long the final notice waits for a still-pending check
// before giving up, so an unusually fast benchmark doesn't block on the network.
const updateNoticeGrace = 1500 * time.Millisecond

// startUpdateCheck launches a non-blocking check for a newer release and returns
// a channel that yields the result (or nil on any error). It is skipped for
// non-release builds (e.g. "dev"), which are never valid semver, so local builds
// are not nagged on every run.
func startUpdateCheck() <-chan *updater.CheckResult {
	ch := make(chan *updater.CheckResult, 1)
	if !semver.IsValid(buildinfo.Version) {
		ch <- nil
		return ch
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		res, err := updater.Check(ctx, buildinfo.Version)
		if err != nil {
			ch <- nil
			return
		}
		ch <- res
	}()
	return ch
}

// autoUpdate acts on the background update check. When a newer release is found
// it prints a notice and updates in place automatically. In a non-interactive
// context (piped/CI) it does not self-modify, printing a passive hint instead so
// scripted runs stay reproducible. It waits at most updateNoticeGrace for a
// still-pending check; a pending or failed check does nothing.
func autoUpdate(ch <-chan *updater.CheckResult) {
	var res *updater.CheckResult
	select {
	case res = <-ch:
	case <-time.After(updateNoticeGrace):
		return // check still running; don't block this run
	}
	if res == nil || !res.HasUpdate {
		return
	}

	m := i18n.L()
	// Non-interactive (piped/CI): just hint, never self-modify unprompted.
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Printf(m.UpdateAvailable, res.Current, res.Latest, res.URL)
		return
	}

	fmt.Printf(m.UpdateAutoNotice, res.Current, res.Latest)
	ctx, cancel := context.WithTimeout(context.Background(), updater.DefaultTimeout)
	defer cancel()
	latest, updated, err := updater.Update(ctx, res.Current)
	if err != nil {
		fmt.Printf("%s %v\n", m.UpdateFailed, err)
		return
	}
	if updated {
		fmt.Printf(m.UpdateDone, latest)
	}
}

func main() {
	// Resolve the language before building commands so that help text honors
	// --lang. Cobra renders help without running PreRun hooks, so the flag is
	// scanned manually here from the raw arguments.
	i18n.Set(i18n.Detect(langFromArgs(os.Args[1:])))
	setup()

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
	}

	// On Windows a double-click (or a launcher like Listary) gives the process
	// its own console that closes the moment it exits, so the user never sees
	// the results. Pause in that case, but not when --json is piped somewhere.
	if !jsonOutput {
		console.PauseOnExit()
	}

	if err != nil {
		os.Exit(1)
	}
}

// langFromArgs extracts the value of --lang from raw CLI arguments, supporting
// both "--lang zh" and "--lang=zh" forms. Returns "" when absent.
func langFromArgs(args []string) string {
	for i, a := range args {
		switch {
		case a == "--lang":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "--lang="):
			return strings.TrimPrefix(a, "--lang=")
		}
	}
	return ""
}
