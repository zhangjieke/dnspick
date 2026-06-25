package ui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/term"

	"github.com/palemoky/dnspick/internal/dnsbench"
	"github.com/palemoky/dnspick/internal/i18n"
)

// catGroup is a group of domains aggregated by category (indices point into
// StatusTracker.domains/done). category is the stable category key.
type catGroup struct {
	category string
	indices  []int
}

// StatusTracker tracks the test progress of every domain and displays it live
// as a categorized table: not-started shows "-", in-progress shows a percentage,
// done shows "✔". On a TTY it refreshes in place; on a non-TTY (pipe/CI) it
// degrades to a static table plus periodic percentages.
type StatusTracker struct {
	mu         sync.Mutex
	domains    []dnsbench.Domain
	idx        map[string]int
	done       []int
	groups     []catGroup // categories displayed side by side
	maxRows    int        // largest group size (determines table row count)
	perTotal   int        // total queries per domain = servers * queries per domain
	grand      int        // total number of queries
	completed  int
	isTTY      bool
	lines      int  // number of lines rendered last time (for in-place TTY refresh)
	lastBucket int  // non-TTY: last printed 10% bucket
	started    bool // whether Start has printed the initial snapshot (non-TTY)
	out        io.Writer
	stop       chan struct{}
	doneCh     chan struct{}
}

func NewStatusTracker(domains []dnsbench.Domain, numServers, queries int) *StatusTracker {
	idx := make(map[string]int, len(domains))
	for i, d := range domains {
		idx[d.Name] = i
	}

	// Aggregate by category, preserving first-seen order, for side-by-side display.
	var order []string
	gmap := make(map[string]*catGroup)
	for i, d := range domains {
		g, ok := gmap[d.Category]
		if !ok {
			g = &catGroup{category: d.Category}
			gmap[d.Category] = g
			order = append(order, d.Category)
		}
		g.indices = append(g.indices, i)
	}
	groups := make([]catGroup, len(order))
	maxRows := 0
	for k, name := range order {
		groups[k] = *gmap[name]
		if n := len(groups[k].indices); n > maxRows {
			maxRows = n
		}
	}

	perTotal := numServers * queries
	return &StatusTracker{
		domains:  domains,
		idx:      idx,
		done:     make([]int, len(domains)),
		groups:   groups,
		maxRows:  maxRows,
		perTotal: perTotal,
		grand:    perTotal * len(domains),
		isTTY:    term.IsTerminal(int(os.Stdout.Fd())),
		out:      color.Output,
	}
}

// Progress is called after each completed query (from multiple goroutines).
func (t *StatusTracker) Progress(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if i, ok := t.idx[domain]; ok {
		t.done[i]++
	}
	t.completed++
	if !t.isTTY && t.grand > 0 {
		if bucket := t.completed * 10 / t.grand; bucket > t.lastBucket {
			t.lastBucket = bucket
			fmt.Fprintf(t.out, i18n.L().ProgressPercent, bucket*10)
		}
	}
}

// Start begins the display. On a TTY it launches a periodic refresh goroutine;
// on a non-TTY it prints the static table once.
func (t *StatusTracker) Start() {
	if !t.isTTY {
		t.printSnapshot()
		t.started = true
		return
	}
	t.draw()
	t.stop = make(chan struct{})
	t.doneCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		defer close(t.doneCh)
		for {
			select {
			case <-t.stop:
				return
			case <-ticker.C:
				t.draw()
			}
		}
	}()
}

// Stop ends the display and performs a final render.
func (t *StatusTracker) Stop() {
	if !t.isTTY {
		if !t.started {
			t.printSnapshot()
		}
		return
	}
	close(t.stop)
	<-t.doneCh
	t.draw()
}

// draw redraws the whole table in place on a TTY.
func (t *StatusTracker) draw() {
	t.mu.Lock()
	lines := t.renderLocked()
	prev := t.lines
	t.lines = len(lines)
	t.mu.Unlock()

	var b strings.Builder
	if prev > 0 {
		fmt.Fprintf(&b, "\033[%dA", prev) // move cursor up prev lines
	}
	for _, ln := range lines {
		b.WriteString(ln)
		b.WriteString("\033[K\n") // clear any leftover at end of line
	}
	fmt.Fprint(t.out, b.String())
}

// printSnapshot prints the current table once on a non-TTY.
func (t *StatusTracker) printSnapshot() {
	t.mu.Lock()
	lines := t.renderLocked()
	t.mu.Unlock()
	fmt.Fprintln(t.out, strings.Join(lines, "\n"))
}

// renderLocked renders the table into a slice of lines (the caller must hold
// the lock). Categories are laid out side by side as column groups (domain |
// status) to reduce vertical height.
func (t *StatusTracker) renderLocked() []string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)

	header := make([]string, 0, len(t.groups)*2)
	for _, g := range t.groups {
		header = append(header, CategoryLabel(g.category), i18n.L().StatusCol)
	}
	table.Header(header)

	for r := range t.maxRows {
		row := make([]string, 0, len(t.groups)*2)
		for _, g := range t.groups {
			if r < len(g.indices) {
				i := g.indices[r]
				row = append(row, t.domains[i].Name, statusCell(t.done[i], t.perTotal))
			} else {
				row = append(row, "", "")
			}
		}
		table.Append(row)
	}
	table.Render()

	pct := 0
	if t.grand > 0 {
		pct = t.completed * 100 / t.grand
	}
	lines := []string{fmt.Sprintf(i18n.L().ProgressLine, pct, t.completed, t.grand)}
	lines = append(lines, strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")...)
	return lines
}

// statusCell returns colored status text based on completion.
func statusCell(done, total int) string {
	switch {
	case done <= 0:
		return color.HiBlackString("-")
	case done >= total:
		return color.GreenString("✔")
	default:
		return color.CyanString("%d%%", done*100/total)
	}
}
