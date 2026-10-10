package main

import (
	"fmt"
	"math"
)

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