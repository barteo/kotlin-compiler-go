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
//
// The files the last update read are statted on the calling goroutine
// (about a microsecond each), and only the files to read go to parallel,
// when there are more than smallCheck. A KSP run's first update comes
// after the processors, with the JVM's threads busy: a stat that blocks
// in a goroutine of its own left the update waiting for a thread to be
// woken, about 1 ms in a third of the runs (step 57).

// filesRead counts the files updates read, for tests.
var filesRead atomic.Int64

// racyWindow is how long after a read a file's stamp isn't trusted; a
// variable for tests.
var racyWindow = 2 * time.Second

// noSerialStat sends every stat to parallel, as before step 57, for
// comparisons.
var noSerialStat = os.Getenv("KC_NO_SERIAL_STAT") == "1"

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
// the next update. A source in fixed (Config.FixedSources: the texts the
// session's last update had) has that text, and its file isn't statted.
func (c *readCache) texts(sources []Source, jobs int, fixed map[string]string) ([]string, []error) {
	c.mu.Lock()
	prev := c.m
	c.mu.Unlock()
	texts := make([]string, len(sources))
	errs := make([]error, len(sources))
	read := make([]readText, len(sources))
	var todo []int
	for i, src := range sources {
		if src.Text != nil {
			texts[i] = *src.Text
			continue
		}
		if t, ok := fixed[src.Path]; ok && !noReuse {
			texts[i] = t
			if e, hit := prev[src.Path]; hit && e.text == t {
				read[i] = e
			}
			continue
		}
		todo = append(todo, i)
	}
	// kept is the last update's read of source i if the file's stamp st
	// (ok if had) says it is the same.
	kept := func(i int, st fileStamp, ok bool) (readText, bool) {
		e, hit := prev[sources[i].Path]
		return e, hit && ok && e.stamp == st && st.changed() < e.readAt-int64(racyWindow)
	}
	// load reads source i, or takes the last update's text if kept; now
	// is before the stat.
	load := func(i int, now int64, st fileStamp, ok bool) {
		src := sources[i]
		if e, same := kept(i, st, ok); same {
			texts[i], read[i] = e.text, e
			return
		}
		texts[i], errs[i] = src.text()
		filesRead.Add(1)
		if ok && errs[i] == nil {
			read[i] = readText{st, now, texts[i], true}
		}
	}
	if noReuse {
		parallel(len(todo), jobs, func(k int) {
			i := todo[k]
			texts[i], errs[i] = sources[i].text()
		})
	} else if noSerialStat {
		parallel(len(todo), jobs, func(k int) {
			now := time.Now().UnixNano()
			st, ok := statStamp(sources[todo[k]].Path)
			load(todo[k], now, st, ok)
		})
	} else {
		// The files the last update had are statted here; the others,
		// and those whose stamps differ, are read, with the stamps had.
		type stat struct {
			i      int
			now    int64
			st     fileStamp
			ok, at bool
		}
		var toRead []stat
		for _, i := range todo {
			if _, hit := prev[sources[i].Path]; !hit {
				toRead = append(toRead, stat{i: i})
				continue
			}
			now := time.Now().UnixNano()
			st, ok := statStamp(sources[i].Path)
			if e, same := kept(i, st, ok); same {
				texts[i], read[i] = e.text, e
				continue
			}
			toRead = append(toRead, stat{i, now, st, ok, true})
		}
		readOne := func(k int) {
			r := toRead[k]
			if !r.at {
				r.now = time.Now().UnixNano()
				r.st, r.ok = statStamp(sources[r.i].Path)
			}
			load(r.i, r.now, r.st, r.ok)
		}
		if len(toRead) > smallCheck {
			parallel(len(toRead), jobs, readOne)
		} else {
			for k := range toRead {
				readOne(k)
			}
		}
	}
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
