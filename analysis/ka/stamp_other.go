//go:build !linux && !darwin

package ka

import "os"

// fileStamp is unused here: without a change time and an inode, a stat
// doesn't say a file is unchanged, so every update reads every file.
type fileStamp struct {
	size  int64
	mtime int64
}

func stampOf(os.FileInfo) (fileStamp, bool) { return fileStamp{}, false }

func (s fileStamp) changed() int64 { return s.mtime }
