package main

import (
	"fmt"
	"math"
)

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

func main() {
	fmt.Println("Monte Carlo Option Pricer")
}