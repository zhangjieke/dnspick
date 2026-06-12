package main

import (
	"strings"
	"testing"
	"time"

	"github.com/palemoky/dns-optimizer/internal/dnsbench"
)

func TestSystemDNSVerdict(t *testing.T) {
	mk := func(name string, score float64, avg time.Duration, succ int, sys bool) dnsbench.Result {
		return dnsbench.Result{Name: name, Address: name, Score: score, AvgTime: avg, Successes: succ, Total: 10, IsSystem: sys}
	}

	t.Run("no system dns", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 100, time.Millisecond, 10, false)}
		if _, ok := systemDNSVerdict(res); ok {
			t.Fatal("expected ok=false when no system DNS")
		}
	})

	t.Run("system is best", func(t *testing.T) {
		res := []dnsbench.Result{mk("sys", 100, time.Millisecond, 10, true), mk("B", 50, 2*time.Millisecond, 10, false)}
		msg, ok := systemDNSVerdict(res)
		if !ok || !strings.Contains(msg, "已是最优") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("system near best", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 100, time.Millisecond, 10, false), mk("sys", 95, time.Millisecond, 10, true)}
		msg, ok := systemDNSVerdict(res)
		if !ok || !strings.Contains(msg, "接近最优") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("system much worse -> switch", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 100, time.Millisecond, 10, false), mk("sys", 40, 5*time.Millisecond, 10, true)}
		msg, ok := systemDNSVerdict(res)
		if !ok || !strings.Contains(msg, "建议切换") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("system all failed -> switch", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 100, time.Millisecond, 10, false), mk("sys", 0, 0, 0, true)}
		msg, ok := systemDNSVerdict(res)
		if !ok || !strings.Contains(msg, "全部失败") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})
}
