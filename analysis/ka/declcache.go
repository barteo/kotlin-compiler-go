package ka

import (
	"runtime"
	"sync"
	"weak"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// declCache keeps the declarations lowered from each parse (a snapshot
// that an update reuses, reuse.go), for as long as the parse is kept: an
// update that checks only some files lowers each of the others' as
// declarations only, and they are the same as the last update's. The
// checker changes the trees it checks, so each update gets a copy
// (syntax.Clone), which costs less than lowering again.
var declCache = struct {
	sync.Mutex
	m map[weak.Pointer[psi.Snapshot]]declEntry
}{m: map[weak.Pointer[psi.Snapshot]]declEntry{}}

type declEntry struct {
	f      *syntax.File // as lowered, never checked
	shares bool         // f holds a node twice (syntax.Clone)
}

// declarations returns lower.Declarations(s), from declCache if s was
// lowered before. A parse with errors is lowered every time.
func declarations(s *psi.Snapshot) (*syntax.File, *lower.Provenance, []*syntax.Error) {
	if noReuse {
		return lower.Declarations(s)
	}
	k := weak.Make(s)
	declCache.Lock()
	e, ok := declCache.m[k]
	declCache.Unlock()
	if ok {
		f, _ := syntax.Clone(e.f, e.shares)
		return f, nil, nil
	}
	f, prov, errs := lower.Declarations(s)
	if len(errs) > 0 {
		return f, prov, errs
	}
	c, shares := syntax.Clone(f, true)
	declCache.Lock()
	if _, ok := declCache.m[k]; !ok {
		declCache.m[k] = declEntry{c, shares}
		runtime.AddCleanup(s, func(k weak.Pointer[psi.Snapshot]) {
			declCache.Lock()
			delete(declCache.m, k)
			declCache.Unlock()
		}, k)
	}
	declCache.Unlock()
	return f, prov, nil
}

// declStub returns, for a file a check of only some files can't reach
// (reachable), a stub of lower.Declarations(s): its path and package,
// which the checker still declares, so that an import of the package
// resolves, but no declarations, which saves copying them. A file whose
// declarations aren't kept, or with file annotations (@file:JvmName,
// @file:JvmMultifileClass) or expect declarations, which pair or clash
// with other files' without naming them, is lowered as declarations.
func declStub(s *psi.Snapshot) (*syntax.File, *lower.Provenance, []*syntax.Error) {
	if noReuse {
		return lower.Declarations(s)
	}
	k := weak.Make(s)
	declCache.Lock()
	e, ok := declCache.m[k]
	declCache.Unlock()
	if !ok {
		return declarations(s)
	}
	f := e.f
	if len(f.Annotations) > 0 || f.JvmName != "" || f.Part != "" || len(f.Expects) > 0 || len(f.Errors) > 0 {
		c, _ := syntax.Clone(f, e.shares)
		return c, nil, nil
	}
	return &syntax.File{Path: f.Path, Package: f.Package, PackageP: f.PackageP, Facade: f.Facade}, nil, nil
}
