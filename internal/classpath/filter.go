package classpath

import "math/bits"

// A name filter answers "is there a class of this name?" without
// building the name: lookups probe names that mostly don't exist, as
// Outer$name for each member a program calls on a library class, and
// pkg/Name for each simple name in each star and default import. The
// caller hashes the parts of a name with Hash and HashJoin, and MayHave
// answers false only for a name that is certainly not on the path; true
// means the class may be there, and the caller looks it up by name.
//
// The filter is a Bloom filter with two probes per name and about 16
// bits per name: about 1.5% of absent names pass it.
type nameFilter struct {
	bits  []uint64
	shift uint // 64 - log2(len(bits)*64)
}

// FNV-1a, 64 bits.
const (
	hashOffset = 14695981039346656037
	hashPrime  = 1099511628211
)

// Hash returns the hash of a class's internal name, or of its first part
// for HashJoin.
func Hash(s string) uint64 { return hashMore(hashOffset, s) }

// HashJoin returns the hash of the name whose first part's hash is h,
// followed by sep and s: HashJoin(Hash("a/B"), '$', "C") is Hash("a/B$C").
func HashJoin(h uint64, sep byte, s string) uint64 {
	h = (h ^ uint64(sep)) * hashPrime
	return hashMore(h, s)
}

func hashMore(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * hashPrime
	}
	return h
}

func newNameFilter(n int) *nameFilter {
	words := 1 << max(4, bits.Len(uint(n*16/64)))
	return &nameFilter{bits: make([]uint64, words), shift: uint(64 - bits.Len(uint(words*64-1)))}
}

// probes returns the two bit positions of a hash.
func (f *nameFilter) probes(h uint64) (uint64, uint64) {
	x := h * 0x9E3779B97F4A7C15
	y := (h ^ h>>29) * 0xBF58476D1CE4E5B9
	return x >> f.shift, y >> f.shift
}

func (f *nameFilter) add(h uint64) {
	a, b := f.probes(h)
	f.bits[a>>6] |= 1 << (a & 63)
	f.bits[b>>6] |= 1 << (b & 63)
}

func (f *nameFilter) has(h uint64) bool {
	a, b := f.probes(h)
	return f.bits[a>>6]&(1<<(a&63)) != 0 && f.bits[b>>6]&(1<<(b&63)) != 0
}

// filter returns the filter of the path's own classes, built on first use.
func (p *Path) filter() *nameFilter {
	p.filterOnce.Do(func() {
		f := newNameFilter(len(p.classes))
		for name := range p.classes {
			f.add(Hash(name))
		}
		p.names = f
	})
	return p.names
}

// MayHave reports whether a class whose internal name hashes to h may be
// on the path: false means Has would report false for it.
func (p *Path) MayHave(h uint64) bool {
	return p.filter().has(h) || p.base != nil && p.base.MayHave(h)
}
