package main

import (
	"fmt"
	"math"
	"testing"
)

func textbook() Params {
	return Params{
		Spot:    100,
		Strike:  100,
		Rate:    0.05,
		Vol:     0.2,
		Years:   1,
		Paths:   50_000,
		Workers: 1,
		Seed:    1,
	}
}


func TestAnalyticCall(t *testing.T) {
	// Textbook case: S = K = 100, r = 5%, σ = 20%, T = 1.
	got, err := AnalyticCall(textbook())
	if err != nil {
		t.Fatal(err)
	}
	const want = 10.450583572185565
	if math.Abs(got-want) > 1e-10 {
		t.Fatalf("analytic = %.16f, want %.16f", got, want)
	}
}

func TestZeroVolMatchesForwardIntrinsic(t *testing.T) {
	p := textbook()
	p.Vol = 0
	p.Paths = 2_000
	want := math.Max(0, p.Spot-p.Strike*math.Exp(-p.Rate*p.Years))

	got, err := AnalyticCall(p)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("analytic = %.16f, want %.16f", got, want)
	}

	for _, mode := range []Mode{ModeSerial, ModePrivate, ModeMutex} {
		res, err := Price(p, mode)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(res.Price-want) > 1e-9 {
			t.Fatalf("%s price = %.16f, want %.16f", mode, res.Price, want)
		}
		if res.StdErr > 1e-12 {
			t.Fatalf("%s stderr = %g, want 0 when volatility is 0", mode, res.StdErr)
		}
	}
}


