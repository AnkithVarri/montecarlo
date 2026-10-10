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