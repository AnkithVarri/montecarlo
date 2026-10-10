package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"
)

// Serial: one loop. Private: one RNG per worker. Mutex: one shared RNG.
type Mode int

const (
	ModeSerial Mode = iota
	ModePrivate
	ModeMutex
)

func (m Mode) String() string {
	switch m {
	case ModeSerial:
		return "serial"
	case ModePrivate:
		return "private"
	case ModeMutex:
		return "mutex"
	default:
		return "unknown"
	}
}

func ParseMode(s string) (Mode, error) {
	switch s {
	case "serial":
		return ModeSerial, nil
	case "private":
		return ModePrivate, nil
	case "mutex":
		return ModeMutex, nil
	default:
		return 0, fmt.Errorf("mode must be serial, private, or mutex")
	}
}

// Params describes a European call and the simulation size.
type Params struct {
	Spot    float64
	Strike  float64
	Rate    float64
	Vol     float64
	Years   float64
	Paths   int
	Workers int // 0 means one worker per CPU
	Seed    int64
}

func (p Params) validate() error {
	if err := finite("spot", p.Spot); err != nil {
		return err
	}
	if err := finite("strike", p.Strike); err != nil {
		return err
	}
	if err := finite("rate", p.Rate); err != nil {
		return err
	}
	if err := finite("vol", p.Vol); err != nil {
		return err
	}
	if err := finite("years", p.Years); err != nil {
		return err
	}
	if p.Spot <= 0 {
		return fmt.Errorf("spot must be positive")
	}
	if p.Strike <= 0 {
		return fmt.Errorf("strike must be positive")
	}
	if p.Vol < 0 {
		return fmt.Errorf("vol must be >= 0")
	}
	if p.Years < 0 {
		return fmt.Errorf("years must be >= 0")
	}
	if p.Paths < 1 {
		return fmt.Errorf("paths must be >= 1")
	}
	if p.Workers < 0 {
		return fmt.Errorf("workers must be >= 0")
	}
	return nil
}

func finite(name string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number", name)
	}
	return nil
}


ype Result struct {
	Mode        Mode
	Price       float64
	StdErr      float64
	Paths       int
	Workers     int
	Elapsed     time.Duration
	PathsPerSec float64
}

func Price(p Params, mode Mode) (Result, error) {
	if err := p.validate(); err != nil {
		return Result{}, err
	}
	switch mode {
	case ModeSerial:
		return priceSerial(p), nil
	case ModePrivate:
		return priceWorkers(p, false), nil
	case ModeMutex:
		return priceWorkers(p, true), nil
	default:
		return Result{}, fmt.Errorf("unknown mode %q", mode)
	}
}

// Closed form, so the simulation has something to miss.
func AnalyticCall(p Params) (float64, error) {
	if err := p.validate(); err != nil {
		return 0, err
	}
	return analytic(p), nil
}

// S_T = S0 * exp((r - σ²/2)T + σ√T Z). Discount is applied once, after the mean.
type model struct {
	spot    float64
	strike  float64
	drift   float64
	diffuse float64
	disc    float64
}

func newModel(p Params) model {
	// Once per price, not once per path.
	return model{
		spot:    p.Spot,
		strike:  p.Strike,
		drift:   (p.Rate - 0.5*p.Vol*p.Vol) * p.Years,
		diffuse: p.Vol * math.Sqrt(p.Years),
		disc:    math.Exp(-p.Rate * p.Years),
	}
}

func priceSerial(p Params) Result {
	m := newModel(p)
	s1, s2 := streamSeed(p.Seed, 0)
	rng := rand.New(rand.NewPCG(s1, s2))

	start := time.Now() // setup stays outside the clock
	sum, sumSq := simulate(m, rng, p.Paths)
	elapsed := time.Since(start)

	return newResult(ModeSerial, m, sum, sumSq, p.Paths, 1, elapsed)
}

func priceWorkers(p Params, shared bool) Result {
	m := newModel(p)
	workers := workerCount(p)
	counts := partition(p.Paths, workers)
	sums := make([]float64, workers)
	sumSqs := make([]float64, workers) // per worker; a shared sum needs a lock

	mode := ModePrivate
	var sharedGen *lockedRNG
	if shared {
		mode = ModeMutex
		s1, s2 := streamSeed(p.Seed, 0) // same stream as serial, so the price matches
		sharedGen = &lockedRNG{r: rand.New(rand.NewPCG(s1, s2))}
	}

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var sum, sumSq float64
			if shared {
				sum, sumSq = simulate(m, sharedGen, counts[i])
			} else {
				s1, s2 := streamSeed(p.Seed, i)
				rng := rand.New(rand.NewPCG(s1, s2))
				sum, sumSq = simulate(m, rng, counts[i])
			}
			sums[i] = sum
			sumSqs[i] = sumSq
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var sum, sumSq float64
	for i := 0; i < workers; i++ {
		sum += sums[i]
		sumSq += sumSqs[i]
	}
	return newResult(mode, m, sum, sumSq, p.Paths, workers, elapsed)
}

// rand.Rand races if two goroutines draw from it.
type lockedRNG struct {
	mu sync.Mutex
	r  *rand.Rand
}

func (g *lockedRNG) NormFloat64() float64 {
	g.mu.Lock()
	z := g.r.NormFloat64()
	g.mu.Unlock()
	return z
}


// Type param so NormFloat64 can inline. An interface can't.
func simulate[R interface{ NormFloat64() float64 }](m model, rng R, n int) (sum, sumSq float64) {
	spot := m.spot
	strike := m.strike
	drift := m.drift
	diffuse := m.diffuse
	for i := 0; i < n; i++ {
		z := rng.NormFloat64()
		sT := spot * math.Exp(drift+diffuse*z)
		payoff := sT - strike
		if payoff < 0 {
			payoff = 0
		}
		sum += payoff
		sumSq += payoff * payoff
	}
	return sum, sumSq
}

func newResult(mode Mode, m model, sum, sumSq float64, n, workers int, elapsed time.Duration) Result {
	mean := sum / float64(n)
	price := m.disc * mean

	var stderr float64
	if n > 1 {
		m2 := sumSq - sum*mean
		if m2 < 1e-12*math.Abs(sumSq) {
			m2 = 0 // zero-vol paths are identical; this is float dust
		}
		variance := m2 / float64(n-1)
		stderr = m.disc * math.Sqrt(variance/float64(n))
	}

	var perSec float64
	if elapsed > 0 {
		perSec = float64(n) / elapsed.Seconds()
	}
	return Result{
		Mode:        mode,
		Price:       price,
		StdErr:      stderr,
		Paths:       n,
		Workers:     workers,
		Elapsed:     elapsed,
		PathsPerSec: perSec,
	}
}

func workerCount(p Params) int {
	n := p.Workers
	if n < 1 {
		n = runtime.NumCPU()
	}
	if n > p.Paths {
		n = p.Paths // extra workers would just get an empty block
	}
	return n
}

func partition(paths, workers int) []int {
	counts := make([]int, workers) // first `extra` workers take the remainder
	base := paths / workers
	extra := paths % workers
	for i := 0; i < workers; i++ {
		counts[i] = base
		if i < extra {
			counts[i]++
		}
	}
	return counts
}

// Separate stream per worker. One shared Rand races.
func streamSeed(base int64, worker int) (uint64, uint64) {
	x := uint64(base) ^ 0xA0761D6478BD642F
	x += uint64(worker) * 0x9E3779B97F4A7C15
	return splitmix64(&x), splitmix64(&x)
}

func splitmix64(x *uint64) uint64 {
	*x += 0x9E3779B97F4A7C15
	z := *x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func analytic(p Params) float64 {
	// d1 divides by σ√T.
	if p.Years == 0 {
		return math.Max(0, p.Spot-p.Strike)
	}
	discK := p.Strike * math.Exp(-p.Rate*p.Years)
	if p.Vol == 0 {
		return math.Max(0, p.Spot-discK)
	}
	sqrtT := math.Sqrt(p.Years)
	d1 := (math.Log(p.Spot/p.Strike) + (p.Rate+0.5*p.Vol*p.Vol)*p.Years) / (p.Vol * sqrtT)
	d2 := d1 - p.Vol*sqrtT
	return p.Spot*normCDF(d1) - discK*normCDF(d2)
}

func normCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2) // Φ(x)
}



