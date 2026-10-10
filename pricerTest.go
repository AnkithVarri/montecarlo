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
