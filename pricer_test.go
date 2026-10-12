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

func TestOneWorkerModesAgree(t *testing.T) {
	// Same seed, one worker: all three modes consume one stream.
	p := textbook()
	p.Paths = 20_000
	p.Workers = 1

	serial, err := Price(p, ModeSerial)
	if err != nil {
		t.Fatal(err)
	}
	private, err := Price(p, ModePrivate)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := Price(p, ModeMutex)
	if err != nil {
		t.Fatal(err)
	}
	if serial.Price != private.Price || serial.Price != shared.Price {
		t.Fatalf("one-worker prices differ: serial %.16f private %.16f mutex %.16f",
			serial.Price, private.Price, shared.Price)
	}
	if private.Workers != 1 || shared.Workers != 1 {
		t.Fatalf("workers = private %d mutex %d, want 1", private.Workers, shared.Workers)
	}
}

func TestParallelNearAnalytic(t *testing.T) {
	p := textbook()
	p.Paths = 200_000
	p.Workers = 4
	res, err := Price(p, ModePrivate)
	if err != nil {
		t.Fatal(err)
	}
	analytic, err := AnalyticCall(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.StdErr <= 0 {
		t.Fatal("expected a positive standard error")
	}
	if math.Abs(res.Price-analytic) > 6*res.StdErr {
		t.Fatalf("price %.6f is %.1f stderr from analytic %.6f",
			res.Price, math.Abs(res.Price-analytic)/res.StdErr, analytic)
	}
	if res.Workers != 4 {
		t.Fatalf("workers = %d, want 4", res.Workers)
	}
}

func TestReproducible(t *testing.T) {
	p := textbook()
	p.Workers = 4
	a, err := Price(p, ModePrivate)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Price(p, ModePrivate)
	if err != nil {
		t.Fatal(err)
	}
	if a.Price != b.Price {
		t.Fatalf("same seed produced %.16f and %.16f", a.Price, b.Price)
	}
}

func TestPartition(t *testing.T) {
	got := partition(10, 4)
	want := []int{3, 3, 2, 2}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	sum := 0
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("partition = %v, want %v", got, want)
		}
		sum += got[i]
	}
	if sum != 10 {
		t.Fatalf("sum = %d, want 10", sum)
	}
}

func TestRejectsBadInputs(t *testing.T) {
	p := textbook()
	p.Spot = 0
	if _, err := Price(p, ModeSerial); err == nil {
		t.Fatal("expected an error for spot 0")
	}
	p = textbook()
	p.Paths = 0
	if _, err := Price(p, ModeSerial); err == nil {
		t.Fatal("expected an error for 0 paths")
	}
	if _, err := ParseMode("gpu"); err == nil {
		t.Fatal("expected an error for an unknown mode")
	}
}

var benchSink float64

func BenchmarkSerial(b *testing.B) {
	benchMode(b, ModeSerial, 1)
}

func BenchmarkPrivate(b *testing.B) {
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(workersName(workers), func(b *testing.B) {
			benchMode(b, ModePrivate, workers)
		})
	}
}

func BenchmarkMutex(b *testing.B) {
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(workersName(workers), func(b *testing.B) {
			benchMode(b, ModeMutex, workers)
		})
	}
}

func workersName(n int) string {
	return fmt.Sprintf("workers=%d", n)
}

func benchMode(b *testing.B, mode Mode, workers int) {
	p := textbook()
	p.Paths = 100_000
	p.Workers = workers
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := Price(p, mode)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = res.Price // otherwise the compiler drops the call
	}
	b.ReportMetric(float64(p.Paths)*float64(b.N)/b.Elapsed().Seconds(), "paths/s")
}
