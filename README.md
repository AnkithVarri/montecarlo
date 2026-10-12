# Monte Carlo Option Pricer

A small Go program that prices a European call option two ways: a Monte Carlo simulation, and the Black–Scholes closed form. The simulation is the estimate. The closed form is the answer it should land near. The gap between them is sampling noise, and the program prints that gap next to a standard error so you can see whether the estimate is actually close.

It also runs the same simulation three ways, so you can see what changes when more than one goroutine draws random numbers.

You need [Go 1.23](https://go.dev/dl/) or newer. No other dependencies.

## Try it

From this directory:

```bash
go run .
```

That prices a one-year at-the-money call: spot 100, strike 100, rate 5%, volatility 20%, 100,000 paths. Workers default to one goroutine per CPU. Same inputs always give the same price, because the seed defaults to 1.

A run looks like this:

```text
European call — Black–Scholes Monte Carlo

  spot           100
  strike         100
  rate           5.00%
  volatility     20.00%
  expiry         1.00 year
  paths          100,000
  mode           private
  workers        10
  seed           1

  monte carlo    10.43xxxx
  std. error     0.04xxxx
  95% interval   [10.xx, 10.xx]
  analytic       10.450584
  difference     -0.0xxxxx

  elapsed        ...
  throughput     ... paths/sec

Within 2 standard errors of the closed-form price.
```

The analytic price for those defaults is about **10.450584**. Your Monte Carlo price will sit nearby. Raise `-paths` if you want it tighter.

Compare the three modes on your machine:

```bash
go run . -bench
```

Change the contract and see the price move:

```bash
go run . -spot 120 -strike 100 -vol 0.3 -years 0.5 -paths 200000
```

`go run . -help` lists every flag.

## What the numbers mean

| Line | Meaning |
| --- | --- |
| monte carlo | Discounted average payoff across the simulated paths. |
| std. error | How much that average typically wobbles. More paths shrink it. |
| 95% interval | Monte Carlo price ± 1.96 standard errors. |
| analytic | Black–Scholes price for the same inputs. |
| difference | Monte Carlo minus analytic. A small difference with a small standard error means the simulation is doing its job. |

The last line says how many standard errors apart the two prices are. "Within 2 standard errors" is the usual result. If it is much farther than that, something is wrong, or you asked for so few paths that the noise is still large.

## Flags

| Flag | Default | What it sets |
| --- | --- | --- |
| `-spot` | `100` | Current price of the underlying. |
| `-strike` | `100` | Strike price. |
| `-rate` | `0.05` | Risk-free rate. `0.05` is 5%, not 0.05%. |
| `-vol` | `0.2` | Volatility. `0.2` is 20%. |
| `-years` | `1` | Time to expiry, in years. |
| `-paths` | `100000` | Number of simulated paths. |
| `-workers` | `0` | Goroutines. `0` means one per CPU. |
| `-seed` | `1` | Random seed. Same seed, same paths, same price. |
| `-mode` | `private` | `serial`, `private`, or `mutex`. |
| `-bench` | off | Print a timing table for all three modes. |

Spot and strike must be positive. Volatility, expiry, and worker count must be zero or positive. Paths must be at least 1.

## The three modes

Each path needs one normal random number. The pricing math is the same in every mode. What changes is who is allowed to draw that number.

**serial** is one loop on one goroutine. It is the baseline the other two are compared with.

**private** gives each worker its own random generator, seeded from the same base seed plus the worker index. Workers never touch each other's state, so this is the fast one. The paths are a different sample from serial, so the price moves by a little sampling noise. That noise is supposed to be there.

**mutex** shares the serial generator and locks it on every draw. The price matches serial exactly, because it consumes the same stream in the same order. Extra workers mostly wait on the lock, so adding them makes it slower. The benchmark is there to show that, not to recommend it.

`-bench` prints serial once, then private and mutex at 1, 2, 4, and 8 workers. Speedup is serial time divided by that row's time. Private should climb. Mutex should fall.

One worker is a special case: all three modes consume one stream, so the prices match exactly.

## How a path is priced

Under Black–Scholes the price at expiry is

```text
S_T = S0 * exp((r − σ²/2) * T + σ * sqrt(T) * Z)
```

`Z` is a standard normal draw. The call pays `max(S_T − K, 0)`. The Monte Carlo price is the average payoff, multiplied once by the discount factor `exp(−rT)`. Nothing is discounted inside the loop.

The closed form is the usual Black–Scholes call. At zero volatility, or at expiry, both sides reduce to the forward intrinsic value, so those cases match exactly rather than statistically.

## Tests and benchmarks

```bash
go test ./...
go test -bench . -benchmem
```

The tests cover the textbook price (spot = strike = 100, rate 5%, volatility 20%, one year, analytic value 10.450583572185565), the zero-volatility case, agreement of the three modes with one worker, a parallel run staying near the analytic price, reproducibility, path splitting, and rejected inputs.

`BenchmarkSerial`, `BenchmarkPrivate`, and `BenchmarkMutex` time 100,000 paths and report a `paths/s` metric. The `-bench` flag on `go run` is the easier comparison if you just want the table.

## Layout

| File | Role |
| --- | --- |
| `main.go` | Flags, printing, and the mode comparison. |
| `pricer.go` | The simulation, the closed form, and the three modes. |
| `pricer_test.go` | Tests and Go benchmarks. |
