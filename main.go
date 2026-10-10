package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func main() {
	p, mode, bench, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if bench {
		if err := runBench(p); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	res, err := Price(p, mode)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	analytic, err := AnalyticCall(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	printPrice(p, res, analytic)
}

func printPrice(p Params, res Result, analytic float64) {
	fmt.Println("European call — Black–Scholes Monte Carlo")
	fmt.Println()
	fmt.Printf("  %-14s %s\n", "spot", fmtInput(p.Spot))
	fmt.Printf("  %-14s %s\n", "strike", fmtInput(p.Strike))
	fmt.Printf("  %-14s %s\n", "rate", fmtPct(p.Rate))
	fmt.Printf("  %-14s %s\n", "volatility", fmtPct(p.Vol))
	fmt.Printf("  %-14s %s\n", "expiry", fmtYears(p.Years))
	fmt.Printf("  %-14s %s\n", "paths", withCommas(p.Paths))
	fmt.Printf("  %-14s %s\n", "mode", res.Mode)
	fmt.Printf("  %-14s %d\n", "workers", res.Workers)
	fmt.Printf("  %-14s %d\n", "seed", p.Seed)
	fmt.Println()
	fmt.Printf("  %-14s %s\n", "monte carlo", fmtPrice(res.Price))
	fmt.Printf("  %-14s %s\n", "std. error", fmtPrice(res.StdErr))
	lo := res.Price - 1.96*res.StdErr // normal 95%
	hi := res.Price + 1.96*res.StdErr
	fmt.Printf("  %-14s [%s, %s]\n", "95% interval", fmtPrice(lo), fmtPrice(hi))
	fmt.Printf("  %-14s %s\n", "analytic", fmtPrice(analytic))
	fmt.Printf("  %-14s %s\n", "difference", fmtSigned(res.Price-analytic))
	fmt.Println()
	fmt.Printf("  %-14s %s\n", "elapsed", fmtDur(res.Elapsed))
	fmt.Printf("  %-14s %s paths/sec\n", "throughput", fmtRate(res.PathsPerSec))
	fmt.Println()
	printAgreement(res.Price, res.StdErr, analytic)
	fmt.Println("Compare serial, private, and mutex with: go run . -bench")
}

func printAgreement(price, stderr, analytic float64) {
	diff := math.Abs(price - analytic)
	switch {
	case stderr == 0 && diff < 1e-9:
		fmt.Println("Matches the closed-form price.")
	case stderr > 0 && diff <= 2*stderr:
		fmt.Println("Within 2 standard errors of the closed-form price.")
	case stderr > 0:
		fmt.Printf("About %.1f standard errors from the closed-form price. Raise -paths for a tighter estimate.\n", diff/stderr)
	default:
		fmt.Println("Closed-form price is printed above as analytic.")
	}
}


func runBench(p Params) error {
	analytic := analytic(p)
	fmt.Println("Black–Scholes Monte Carlo")
	fmt.Println()
	fmt.Printf("  spot %s    strike %s    rate %s    vol %s    expiry %s\n",
		fmtInput(p.Spot), fmtInput(p.Strike), fmtPct(p.Rate), fmtPct(p.Vol), fmtYears(p.Years))
	fmt.Printf("  paths %s    seed %d    CPUs %d\n", withCommas(p.Paths), p.Seed, runtime.NumCPU())
	fmt.Printf("  analytic %s\n", fmtPrice(analytic))
	fmt.Println()

	warm := p // first row was cold and looked slower than it is
	warm.Paths = min(p.Paths, 50_000)
	warm.Workers = 2
	if _, err := Price(warm, ModePrivate); err != nil {
		return err
	}

	fmt.Printf("%-8s  %7s  %12s  %10s  %9s  %12s\n",
		"mode", "workers", "time", "paths/sec", "speedup", "price")

	serial := p
	serial.Workers = 1
	base, err := Price(serial, ModeSerial)
	if err != nil {
		return err
	}
	printBenchRow(base, base.Elapsed)

	for _, workers := range []int{1, 2, 4, 8} {
		if workers > p.Paths {
			continue // would clamp and repeat a smaller row
		}
		run := p
		run.Workers = workers
		res, err := Price(run, ModePrivate)
		if err != nil {
			return err
		}
		printBenchRow(res, base.Elapsed)
	}
	for _, workers := range []int{1, 2, 4, 8} {
		if workers > p.Paths {
			continue
		}
		run := p
		run.Workers = workers
		res, err := Price(run, ModeMutex)
		if err != nil {
			return err
		}
		printBenchRow(res, base.Elapsed)
	}

	fmt.Println()
	fmt.Println("Private gives each worker its own generator, so those prices differ by sampling noise.")
	fmt.Println("Mutex shares one generator and locks every draw. The price matches serial; extra workers add contention.")
	fmt.Println("Speedup is serial time divided by that row's time.")
	return nil
}

func printBenchRow(res Result, serial time.Duration) {
	speedup := 0.0
	if res.Elapsed > 0 {
		speedup = float64(serial) / float64(res.Elapsed)
	}
	fmt.Printf("%-8s  %7d  %12s  %10s  %8.2fx  %12s\n",
		res.Mode, res.Workers, fmtDur(res.Elapsed), fmtRate(res.PathsPerSec), speedup, fmtPrice(res.Price))
}


func fmtPrice(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

func fmtInput(v float64) string {
	s := strconv.FormatFloat(v, 'f', 6, 64) // then strip 100.000000 -> 100
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func fmtSigned(v float64) string {
	return fmt.Sprintf("%+.6f", v)
}

func fmtPct(v float64) string {
	return fmt.Sprintf("%.2f%%", v*100)
}

func fmtYears(v float64) string {
	if v == 1 {
		return "1.00 year"
	}
	return fmt.Sprintf("%.2f years", v)
}

func fmtDur(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.1f µs", float64(d)/float64(time.Microsecond))
	case d < time.Second:
		return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2f s", d.Seconds())
	}
}

func fmtRate(v float64) string {
	switch {
	case v >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.2fK", v/1e3)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

func withCommas(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.Itoa(n)
	lead := len(s) % 3
	if lead == 0 {
		lead = 3
	}
	var b strings.Builder
	b.WriteString(s[:lead])
	for i := lead; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return sign + b.String()
}
