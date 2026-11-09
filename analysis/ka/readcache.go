package ka

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Read sources. A library keeps the texts the updates against it read
// from disk (shared.read: a daemon's KSP runs each open a session),
// each with the file's stamp when it was read (fileStamp: size,
// modification and change times, inode). An update stats each file and
// reads only those whose stamps differ, as git's index does: a daemon's
// update after a one-file edit reads that file, not the module. A file
// changed within racyWindow of being read (its modification or change
// time) may have changed again without its times moving (a coarse clock),
// so its text is read again until it is older than that.

// filesRead counts the files updates read, for tests.
var filesRead atomic.Int64

// racyWindow is how long after a read a file's stamp isn't trusted; a
// variable for tests.
var racyWindow = 2 * time.Second

// readText is a source's text as read, and the file's stamp then.
type readText struct {
	stamp  fileStamp
	readAt int64 // nanoseconds since the epoch, before the stat
	text   string
	kept   bool
}

// readCache is a session's read texts, by path.
type readCache struct {
	mu sync.Mutex
	m  map[string]readText
}

// texts returns the texts of sources, reading the files of those without
// a Text whose stamps differ from the last update's, and keeps them for
// the next update.
func (c *readCache) texts(sources []Source, jobs int) ([]string, []error) {
	c.mu.Lock()
	prev := c.m
	c.mu.Unlock()
	texts := make([]string, len(sources))
	errs := make([]error, len(sources))
	read := make([]readText, len(sources))
	parallel(len(sources), jobs, func(i int) {
		src := sources[i]
		if src.Text != nil {
			texts[i] = *src.Text
			return
		}
		if noReuse {
			texts[i], errs[i] = src.text()
			return
		}
		now := time.Now().UnixNano()
		st, ok := statStamp(src.Path)
		if e, hit := prev[src.Path]; hit && ok && e.stamp == st && st.changed() < e.readAt-int64(racyWindow) {
			texts[i], read[i] = e.text, e
			return
		}
		texts[i], errs[i] = src.text()
		filesRead.Add(1)
		if ok && errs[i] == nil {
			read[i] = readText{st, now, texts[i], true}
		}
	})
	m := make(map[string]readText, len(sources))
	for i, src := range sources {
		if read[i].kept {
			m[src.Path] = read[i]
		}
	}
	c.mu.Lock()
	c.m = m
	c.mu.Unlock()
	return texts, errs
}

// statStamp returns path's stamp, or false if it can't be had.
func statStamp(path string) (fileStamp, bool) {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return fileStamp{}, false
	}
	return stampOf(fi)
}
