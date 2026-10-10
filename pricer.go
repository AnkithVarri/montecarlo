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