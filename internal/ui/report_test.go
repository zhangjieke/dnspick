package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/palemoky/dnspick/internal/dnsbench"
)

func TestSystemDNSVerdict(t *testing.T) {
	// mk builds a result from avg latency (ms) and success rate; Successes is derived from the rate.
	mk := func(name string, avgMs int, rate float64, sys bool) dnsbench.Result {
		return dnsbench.Result{
			Name: name, Address: name,
			AvgTime:     time.Duration(avgMs) * time.Millisecond,
			SuccessRate: rate, Successes: int(rate * 10), Total: 10, IsSystem: sys,
		}
	}

	t.Run("no system dns", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 5, 1.0, false)}
		if _, ok := systemDNSVerdict(res); ok {
			t.Fatal("expected ok=false when no system DNS")
		}
	})

	t.Run("system is best", func(t *testing.T) {
		res := []dnsbench.Result{mk("sys", 5, 1.0, true), mk("B", 10, 1.0, false)}
		if msg, ok := systemDNSVerdict(res); !ok || !strings.Contains(msg, "is already the best") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})

	// User scenario: only 2ms slower should NOT recommend switching.
	t.Run("system 2ms slower -> no switch", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 5, 1.0, false), mk("sys", 7, 1.0, true)}
		msg, ok := systemDNSVerdict(res)
		if !ok || strings.Contains(msg, "consider switching") {
			t.Fatalf("2ms gap should NOT recommend switching; got ok=%v msg=%q", ok, msg)
		}
		if !strings.Contains(msg, "no change needed") {
			t.Fatalf("expected \"no change needed\", got %q", msg)
		}
	})

	t.Run("system much slower -> switch", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 5, 1.0, false), mk("sys", 25, 1.0, true)}
		if msg, ok := systemDNSVerdict(res); !ok || !strings.Contains(msg, "consider switching") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})

	// Close latency but clearly worse reliability should also recommend switching.
	t.Run("system flaky -> switch", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 5, 1.0, false), mk("sys", 6, 0.7, true)}
		if msg, ok := systemDNSVerdict(res); !ok || !strings.Contains(msg, "consider switching") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("system all failed -> switch", func(t *testing.T) {
		res := []dnsbench.Result{mk("A", 5, 1.0, false), mk("sys", 0, 0, true)}
		if msg, ok := systemDNSVerdict(res); !ok || !strings.Contains(msg, "failed every query") {
			t.Fatalf("got ok=%v msg=%q", ok, msg)
		}
	})
}
