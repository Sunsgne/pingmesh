package main

import (
	"testing"
	"time"
)

func TestClockSteppedBack(t *testing.T) {
	prev := time.Date(2026, 11, 5, 13, 17, 40, 0, time.Local)
	now := time.Date(2026, 11, 5, 6, 24, 50, 0, time.Local)
	if !clockSteppedBack(prev, now, 10*time.Second) {
		t.Fatal("a multi-hour step backward must restart the scheduler")
	}
	if clockSteppedBack(prev, prev.Add(2*time.Second), 2*time.Second) {
		t.Fatal("a normal tick is not a step backward")
	}
	if clockSteppedBack(prev, prev.Add(time.Hour), 2*time.Second) {
		t.Fatal("a forward step still lets cron catch up")
	}
	if clockSteppedBack(prev, prev, 0) {
		t.Fatal("zero elapsed is not a step")
	}
}
