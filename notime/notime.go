package notime

import (
	"time"
	"unsafe"
	_ "unsafe" // Required for go:linkname
)

//go:linkname newTimer time.newTimer
func newTimer(when, period int64, f func(any, uintptr, int64), arg any, cp unsafe.Pointer) *time.Timer {
	panic("new timer")
}

//go:linkname stopTimer
func stopTimer(*time.Timer) bool {
	panic("stop timer")
}

//go:linkname resetTimer
func resetTimer(t *time.Timer, when, period int64) bool {
	panic("reset timer")
}

func absClock(abs uint64) (hour, min, sec int) {
	panic("abs clock")
}

//go:linkname absDate
func absDate(abs uint64, full bool) (year int, month time.Month, day int, yday int) {
	panic("abs date")
}

//go:linkname runtimeNano
func runtimeNano() int64 {
	panic("runtime nano")
}
