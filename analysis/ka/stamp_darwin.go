package ka

import (
	"os"
	"syscall"
	"time"
)

// fileStamp is what a file's stat says of its contents: a write moves its
// change time, and a rename over it its inode.
type fileStamp struct {
	size         int64
	mtime, ctime int64 // nanoseconds since the epoch
	dev, ino     uint64
}

func stampOf(fi os.FileInfo) (fileStamp, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fileStamp{}, false
	}
	return fileStamp{fi.Size(), fi.ModTime().UnixNano(), time.Unix(st.Ctimespec.Unix()).UnixNano(), uint64(st.Dev), st.Ino}, true
}

// changed is the later of the file's modification and change times.
func (s fileStamp) changed() int64 { return max(s.mtime, s.ctime) }
