package ka

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Timing is set when the environment has KC_TIMING: Update then writes how
// long each of its phases took to standard error, on a line that starts
// with "kc-go:" (bench/ksp).
var Timing = os.Getenv("KC_TIMING") != ""

// A Timer measures consecutive phases; a nil Timer measures nothing.
type Timer struct {
	last  time.Time
	parts []string
}

// NewTimer returns a Timer started now if Timing is set, and nil if not.
func NewTimer() *Timer {
	if !Timing {
		return nil
	}
	return &Timer{last: time.Now()}
}

// Mark ends the phase called name, which started at the last mark.
func (t *Timer) Mark(name string) {
	if t == nil {
		return
	}
	t.MarkAt(name, time.Now())
}

// MarkAt ends the phase called name at end.
func (t *Timer) MarkAt(name string, end time.Time) {
	if t == nil {
		return
	}
	t.parts = append(t.parts, fmt.Sprintf("%s=%.1f", name, float64(end.Sub(t.last).Microseconds())/1000))
	t.last = end
}

// Note adds a count, called name, to the phases.
func (t *Timer) Note(name string, n int) {
	if t == nil {
		return
	}
	t.parts = append(t.parts, fmt.Sprintf("%s=%d", name, n))
}

// Print writes the phases, in ms, after what.
func (t *Timer) Print(what string) {
	if t == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "kc-go: %s %s\n", what, strings.Join(t.parts, " "))
}
