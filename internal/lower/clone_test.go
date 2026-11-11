package lower

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestClone clones the trees that File and Declarations lower from the
// test sources: a copy must equal its tree and share none of its nodes,
// slices or maps, with or without the copies' map.
func TestClone(t *testing.T) {
	n, sharing := 0, 0
	err := filepath.WalkDir("../../testdata", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, lowerer := range []func(*psi.Snapshot) (*syntax.File, *Provenance, []*syntax.Error){File, Declarations} {
			f, _, _ := lowerer(psi.Parse(path, string(src)))
			n++
			c, shares := syntax.Clone(f, true)
			if shares {
				sharing++
			}
			if d := syntax.Diff(f, c, false); d != "" {
				t.Errorf("%s: copy differs: %s", path, d)
			}
			if p := sharedMemory(f, c); p != "" {
				t.Errorf("%s: copy shares %s", path, p)
			}
			if !shares {
				c, _ := syntax.Clone(f, false)
				if d := syntax.Diff(f, c, false); d != "" {
					t.Errorf("%s: copy without the map differs: %s", path, d)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sharing == 0 {
		t.Errorf("no tree of %d holds a node twice", n)
	}
}

// sharedMemory returns the path of a pointer, slice or map in b that is
// also in a, or "".
func sharedMemory(a, b *syntax.File) string {
	in := map[uintptr]bool{}
	walkMemory(reflect.ValueOf(a), "", map[uintptr]bool{}, func(p uintptr, _ string) { in[p] = true })
	found := ""
	walkMemory(reflect.ValueOf(b), "File", map[uintptr]bool{}, func(p uintptr, path string) {
		if in[p] && found == "" {
			found = path
		}
	})
	return found
}

func walkMemory(v reflect.Value, path string, seen map[uintptr]bool, note func(uintptr, string)) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		note(v.Pointer(), path)
		if seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		walkMemory(v.Elem(), path, seen, note)
	case reflect.Interface:
		if !v.IsNil() {
			walkMemory(v.Elem(), path, seen, note)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			walkMemory(v.Field(i), path+"."+v.Type().Field(i).Name, seen, note)
		}
	case reflect.Slice:
		if v.Cap() > 0 {
			note(v.Pointer(), path)
		}
		for i := range v.Len() {
			walkMemory(v.Index(i), path+"[]", seen, note)
		}
	case reflect.Array:
		for i := range v.Len() {
			walkMemory(v.Index(i), path+"[]", seen, note)
		}
	case reflect.Map:
		if v.IsNil() {
			return
		}
		note(v.Pointer(), path)
		it := v.MapRange()
		for it.Next() {
			walkMemory(it.Key(), path+"{key}", seen, note)
			walkMemory(it.Value(), path+"{}", seen, note)
		}
	}
}
