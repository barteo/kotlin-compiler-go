package ka

import (
	"os"
	"sync"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
)

// Parsed Java sources. A library keeps the parse of each Java source the
// last update against it had (shared.java), by path, with its text: a
// KSP run's second round, and a daemon's next run, parse only the Java
// sources whose texts changed. A parsed file is only read once parsed
// (javasrc.NewSet and the class views it gives build values of their
// own), so models can share it.

// noRoundReuse turns off the reuse of a KSP run's last round's sources:
// the Java parses kept here and the texts of Config.FixedSources.
var noRoundReuse = os.Getenv("KC_NO_ROUND_REUSE") == "1"

// javaParsed counts the Java sources updates parsed, for tests.
var javaParsed atomic.Int64

type javaParse struct {
	text string
	file *javasrc.File
	err  error
}

// javaCache is a library's parsed Java sources, by path.
type javaCache struct {
	mu sync.Mutex
	m  map[string]javaParse
}

// parse returns the parses of sources, whose texts are texts, parsing
// those whose texts differ from the last update's, and keeps them for the
// next update.
func (c *javaCache) parse(sources []Source, texts []string, jobs int) ([]*javasrc.File, []error) {
	c.mu.Lock()
	prev := c.m
	c.mu.Unlock()
	files := make([]*javasrc.File, len(sources))
	errs := make([]error, len(sources))
	var todo []int
	for i, src := range sources {
		if p, ok := prev[src.Path]; ok && p.text == texts[i] && !noReuse && !noRoundReuse {
			files[i], errs[i] = p.file, p.err
			continue
		}
		todo = append(todo, i)
	}
	parallel(len(todo), jobs, func(k int) {
		i := todo[k]
		files[i], errs[i] = javasrc.Parse(sources[i].Path, texts[i])
		javaParsed.Add(1)
	})
	m := make(map[string]javaParse, len(sources))
	for i, src := range sources {
		m[src.Path] = javaParse{texts[i], files[i], errs[i]}
	}
	c.mu.Lock()
	c.m = m
	c.mu.Unlock()
	return files, errs
}
