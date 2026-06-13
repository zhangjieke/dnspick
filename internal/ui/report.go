package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"

	"github.com/palemoky/dnspick/internal/dnsbench"
	"github.com/palemoky/dnspick/internal/i18n"
)

// PrintResultsTable prints a formatted result table using tablewriter.
func PrintResultsTable(results []dnsbench.Result) {
	table := tablewriter.NewWriter(os.Stdout)
	table.Header(i18n.L().TableCol)

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	for i, r := range results {
		rateStr := fmt.Sprintf("%.1f%% (%d/%d)", r.SuccessRate*100, r.Successes, r.Total)
		if r.SuccessRate < 1.0 {
			rateStr = red(rateStr)
		} else {
			rateStr = green(rateStr)
		}

		name := r.Name
		if r.IsSystem {
			name += i18n.L().SystemSuffix
		}
		table.Append([]string{
			fmt.Sprintf("%d", i+1),
			name,
			r.Address,
			r.AvgTime.Round(time.Microsecond).String(),
			rateStr,
			fmt.Sprintf("%.2f", r.Score),
		})
	}
	table.Render()
}

// PrintRecommendations prints the top recommendations.
func PrintRecommendations(results []dnsbench.Result) {
	palette := []*color.Color{
		color.New(color.FgGreen, color.Bold),
		color.New(color.FgYellow),
		color.New(color.FgCyan),
	}
	red := color.New(color.FgRed)

	found := 0
	for _, best := range results {
		if best.SuccessRate <= 0.98 {
			continue
		}
		palette[found].Printf("#%d: %s (%s)\n", found+1, best.Name, best.Address)
		fmt.Printf(i18n.L().RecommendLine,
			best.Score, best.AvgTime.Round(time.Microsecond).String(), best.SuccessRate*100)
		found++
		if found >= len(palette) {
			break
		}
	}
	if found == 0 {
		red.Println(i18n.L().NoGoodDNS)
	}

	if msg, ok := systemDNSVerdict(results); ok {
		c := color.New(color.FgGreen, color.Bold)
		if strings.HasPrefix(msg, "⚠") {
			c = color.New(color.FgYellow, color.Bold)
		}
		fmt.Println()
		c.Println(msg)
	}
}

const (
	// switchLatencyThreshold: if the system DNS is slower than the best by less
	// than this, the gap is treated as insignificant and no switch is suggested.
	switchLatencyThreshold = 15 * time.Millisecond
	// switchSuccessMargin: if the system DNS success rate trails the best by more
	// than this, a switch is suggested even when latency is close.
	switchSuccessMargin = 0.05
)

// systemDNSVerdict produces a conclusion on whether the system default DNS
// should be changed. A switch is only suggested when the system DNS is clearly
// slower (latency gap >= switchLatencyThreshold) or clearly less reliable,
// avoiding misleading the user over a few insignificant milliseconds. If the
// results contain no system DNS, ok is false. results must be sorted by score
// in descending order.
func systemDNSVerdict(results []dnsbench.Result) (msg string, ok bool) {
	if len(results) == 0 {
		return "", false
	}
	sysRank := -1
	for i := range results {
		if results[i].IsSystem {
			sysRank = i
			break
		}
	}
	if sysRank < 0 {
		return "", false
	}

	sys := results[sysRank]
	best := results[0]
	latencyGap := sys.AvgTime - best.AvgTime
	closeEnough := latencyGap < switchLatencyThreshold && best.SuccessRate-sys.SuccessRate <= switchSuccessMargin

	m := i18n.L()
	switch {
	case sys.Successes == 0:
		return fmt.Sprintf(m.VerdictAllFailed, sys.Address, best.Name, best.Address), true
	case sysRank == 0:
		return fmt.Sprintf(m.VerdictBest, sys.Address), true
	case closeEnough:
		return fmt.Sprintf(m.VerdictGoodEnough, sys.Address, sysRank+1, latencyGap.Round(time.Microsecond)), true
	default:
		return fmt.Sprintf(m.VerdictSwitch, sys.Address, sysRank+1, best.Name, best.Address,
			sys.AvgTime.Round(time.Microsecond), best.AvgTime.Round(time.Microsecond)), true
	}
}
