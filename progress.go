package main

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

	"github.com/palemoky/dns-optimizer/internal/dnsbench"
)

// statusTracker 维护每个域名的测试进度，并以分类表格的形式实时展示：
// 未开始显示 "-"，进行中显示百分比，完成显示 "✔"。
// 在 TTY 下原地刷新；非 TTY（管道/CI）下降级为静态表 + 周期性百分比。
type statusTracker struct {
	mu         sync.Mutex
	domains    []dnsbench.Domain
	idx        map[string]int
	done       []int
	perTotal   int // 单个域名的总查询次数 = 服务器数 * 每域查询次数
	grand      int // 所有查询总数
	completed  int
	isTTY      bool
	lines      int // 上一次渲染的行数（TTY 原地刷新用）
	lastBucket int // 非 TTY：上次打印的 10% 档位
	out        io.Writer
	stop       chan struct{}
	doneCh     chan struct{}
}

func newStatusTracker(domains []dnsbench.Domain, numServers, queries int) *statusTracker {
	idx := make(map[string]int, len(domains))
	for i, d := range domains {
		idx[d.Name] = i
	}
	perTotal := numServers * queries
	return &statusTracker{
		domains:  domains,
		idx:      idx,
		done:     make([]int, len(domains)),
		perTotal: perTotal,
		grand:    perTotal * len(domains),
		isTTY:    term.IsTerminal(int(os.Stdout.Fd())),
		out:      color.Output,
	}
}

// Progress 在每次查询完成后被调用（来自多个 goroutine）。
func (t *statusTracker) Progress(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if i, ok := t.idx[domain]; ok {
		t.done[i]++
	}
	t.completed++
	if !t.isTTY && t.grand > 0 {
		if bucket := t.completed * 10 / t.grand; bucket > t.lastBucket {
			t.lastBucket = bucket
			fmt.Fprintf(t.out, "  测试进度: %d%%\n", bucket*10)
		}
	}
}

// Start 开始展示。TTY 下启动定时刷新协程；非 TTY 下打印一次静态表。
func (t *statusTracker) Start() {
	if !t.isTTY {
		t.printSnapshot()
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

// Stop 结束展示并做最终渲染。
func (t *statusTracker) Stop() {
	if !t.isTTY {
		t.printSnapshot()
		return
	}
	close(t.stop)
	<-t.doneCh
	t.draw()
}

// draw 在 TTY 下原地重绘整张表。
func (t *statusTracker) draw() {
	t.mu.Lock()
	lines := t.renderLocked()
	prev := t.lines
	t.lines = len(lines)
	t.mu.Unlock()

	var b strings.Builder
	if prev > 0 {
		fmt.Fprintf(&b, "\033[%dA", prev) // 光标上移 prev 行
	}
	for _, ln := range lines {
		b.WriteString(ln)
		b.WriteString("\033[K\n") // 清除行尾残留
	}
	fmt.Fprint(t.out, b.String())
}

// printSnapshot 非 TTY 下一次性打印当前表格。
func (t *statusTracker) printSnapshot() {
	t.mu.Lock()
	lines := t.renderLocked()
	t.mu.Unlock()
	fmt.Fprintln(t.out, strings.Join(lines, "\n"))
}

// renderLocked 渲染表格为行切片（调用方须持有锁）。
func (t *statusTracker) renderLocked() []string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"分类", "域名", "状态"})

	prevCat := ""
	for i, d := range t.domains {
		cat := d.Category
		if cat == prevCat {
			cat = "" // 同组只在首行显示分类
		} else {
			prevCat = d.Category
		}
		table.Append([]string{cat, d.Name, statusCell(t.done[i], t.perTotal)})
	}
	table.Render()

	pct := 0
	if t.grand > 0 {
		pct = t.completed * 100 / t.grand
	}
	lines := []string{fmt.Sprintf("测试进度: %d%% (%d/%d)", pct, t.completed, t.grand)}
	lines = append(lines, strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")...)
	return lines
}

// statusCell 根据完成情况返回带颜色的状态文本。
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
