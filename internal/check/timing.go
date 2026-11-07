package check

import (
	"fmt"
	"io"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// phaseTimer writes the times of the checker's phases for -time, each as
// a line "name duration (part of check: note)", as the driver writes its
// own, with the class files the phase loaded and the Kotlin declarations
// it decoded from metadata. Those include what the default imports'
// prefetch, running concurrently, loads in the meantime. It does nothing
// without a writer.
type phaseTimer struct {
	w     io.Writer
	cp    *classpath.Path
	start time.Time
	last  loadStats
}

// loadStats are the class path's counts of classes loaded and Kotlin
// declarations decoded, and the time spent on each.
type loadStats struct {
	classes, decls      int
	classTime, declTime time.Duration
}

func (t *phaseTimer) stats() loadStats {
	var s loadStats
	if t.cp != nil {
		s.classes, s.classTime = t.cp.Loaded()
		s.decls, s.declTime = t.cp.Kotlin.Get()
	}
	return s
}

func newPhaseTimer(w io.Writer, cp *classpath.Path) *phaseTimer {
	t := &phaseTimer{w: w, cp: cp}
	if w != nil {
		t.last = t.stats()
	}
	t.start = time.Now()
	return t
}

// mark writes the time since the last mark as phase name's.
func (t *phaseTimer) mark(name, format string, args ...any) {
	if t.w == nil {
		return
	}
	now := time.Now()
	note := fmt.Sprintf(format, args...)
	s := t.stats()
	if n := s.classes - t.last.classes; n > 0 {
		note += fmt.Sprintf("; loaded %d classes, %v summed over goroutines", n, (s.classTime - t.last.classTime).Round(time.Microsecond))
	}
	if n := s.decls - t.last.decls; n > 0 {
		note += fmt.Sprintf("; decoded %d Kotlin declarations, %v summed", n, (s.declTime - t.last.declTime).Round(time.Microsecond))
	}
	fmt.Fprintf(t.w, "%-8s %v (part of check: %s)\n", name, now.Sub(t.start).Round(time.Microsecond), note)
	t.last = s
	t.start = time.Now()
}

// pendingInfer counts the file's declarations whose omitted types
// inferAll is still to infer.
func (c *Checker) pendingInfer() int {
	n := 0
	count := func(d syntax.Decl) {
		switch d := d.(type) {
		case *syntax.FunDecl:
			if f := d.Sym; f != nil && !f.Resolved && f.Infer != nil {
				n++
			}
		case *syntax.PropDecl:
			if v := d.Sym; v != nil && !v.Resolved && v.Infer != nil {
				n++
			}
		}
	}
	for _, d := range c.file.Decls {
		count(d)
	}
	for _, d := range c.classes {
		for _, m := range d.Members {
			count(m)
		}
	}
	return n
}

// bodyTimes are the times of the checkers' checkBodies, for -time.
type bodyTimes struct {
	total    time.Duration
	slowest  time.Duration
	slowFile string
}

func (b *bodyTimes) add(c *Checker, d time.Duration) {
	b.total += d
	if d > b.slowest {
		b.slowest, b.slowFile = d, c.file.Path
	}
}
