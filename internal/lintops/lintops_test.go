package lintops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
)

// decoded is a file's facts as the JVM reads them.
type decoded struct {
	types []dtype
	syms  map[uint64]dsym
	facts map[uint64]dfact // by node
}

type dtype struct {
	name  string
	flags uint64
	args  []uint64
}

type dsym struct {
	kind uint64
	name string
	typ  uint64
	decl [2]uint64
}

type dfact struct {
	role, typ, sym uint64
	call           *dcall
	flags          uint64
	ret            [2]uint64
}

type dcall struct {
	name    string
	decl    [2]uint64
	params  []string
	mapping []int64
	ret     uint64
	targets []*dcall
}

func readElem(r *wire.Reader) [2]uint64 {
	f := r.Uint()
	if f == 0 {
		return [2]uint64{}
	}
	return [2]uint64{f, r.Uint()}
}

func readStrs(r *wire.Reader) []string {
	var out []string
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		out = append(out, r.String())
	}
	return out
}

func readCall(r *wire.Reader) *dcall {
	c := &dcall{name: r.String()}
	_ = r.String() // owner class id
	c.decl = readElem(r)
	r.Uint()    // cflags
	r.Uint()    // declared receiver
	r.Uint()    // receiver
	readElem(r) // implicit receiver
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		r.Uint()
		c.params = append(c.params, r.String())
		r.Uint()
	}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		c.mapping = append(c.mapping, r.Int())
	}
	c.ret = r.Uint()
	r.Uint()    // declared return type
	r.Uint()    // type parameters
	readStrs(r) // type parameter names
	readStrs(r) // annotations
	readStrs(r) // owner annotations
	readStrs(r) // package annotations
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		c.targets = append(c.targets, readCall(r))
	}
	return c
}

func decode(t *testing.T, msg []byte) decoded {
	t.Helper()
	r := wire.NewReader(msg)
	d := decoded{syms: map[uint64]dsym{}, facts: map[uint64]dfact{}}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		ty := dtype{name: r.String()}
		_ = r.String() // class id
		ty.flags = r.Uint()
		readElem(r)
		r.Uint() // projection
		for m := r.Uint(); m > 0 && r.Err() == nil; m-- {
			a := r.Uint()
			if int(a) >= len(d.types) {
				t.Fatalf("type argument %d refers forward", a)
			}
			ty.args = append(ty.args, a)
		}
		readStrs(r)
		d.types = append(d.types, ty)
	}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		id := r.Uint()
		s := dsym{kind: r.Uint(), name: r.String()}
		_ = r.String()
		_ = r.String()
		s.typ = r.Uint()
		s.decl = readElem(r)
		readStrs(r)
		r.Uint()
		d.syms[id] = s
	}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		node, mask := r.Uint(), r.Uint()
		var f dfact
		has := func(bit int) bool { return mask&(1<<bit) != 0 }
		if has(0) {
			f.role = r.Uint()
		}
		if has(1) {
			f.typ = r.Uint() + 1
		}
		if has(2) {
			f.sym = r.Uint()
		}
		if has(3) {
			f.call = readCall(r)
		}
		for bit := 4; bit <= 6; bit++ {
			if has(bit) {
				_ = r.String()
			}
		}
		if has(7) {
			r.Bool()
			_ = r.String()
			readStrs(r)
		}
		if has(8) {
			r.Bool()
		}
		if has(9) {
			for m := r.Uint(); m > 0 && r.Err() == nil; m-- {
				r.Uint()
			}
		}
		if has(10) {
			f.ret = readElem(r)
		}
		if has(11) {
			readElem(r)
		}
		if has(12) {
			readElem(r)
		}
		if has(13) {
			r.Uint()
		}
		for bit := 14; bit <= 15; bit++ {
			if has(bit) {
				for m := r.Uint(); m > 0 && r.Err() == nil; m-- {
					readCall(r)
				}
			}
		}
		if has(16) {
			_ = r.String()
		}
		if has(17) {
			f.flags = r.Uint()
		}
		d.facts[node] = f
	}
	if r.Err() != nil || r.Len() != 0 {
		t.Fatalf("decode: err %v, %d bytes left", r.Err(), r.Len())
	}
	return d
}

func TestAnalyzeFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.kt")
	b := filepath.Join(dir, "b.kt")
	if err := os.WriteFile(a, []byte("package p\nfun f(x: Int): Int = g(x, 1)\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("package p\nfun g(a: Int, b: Int): Int {\n    return a\n}\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	var w wire.Writer
	w.String("")
	w.Bool(true) // no JDK
	w.String("m")
	w.Uint(1)
	w.Uint(0) // no class path
	w.Uint(0) // no friend paths
	w.Uint(0) // no opt-ins
	w.Uint(2)
	w.String(a)
	w.String(b)
	resp, err := kcabi.Call(OpLintAnalyze, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	r := wire.NewReader(resp)
	h := r.Int()
	if n := r.Uint(); n != 0 {
		t.Fatalf("%d diagnostics: %s:%d: %s", n, r.String(), r.Uint(), func() string { r.Uint(); _ = r.String(); return r.String() }())
	}
	if n := r.Uint(); n != 2 {
		t.Fatalf("%d files", n)
	}
	for range 2 {
		if nodes, _ := r.Uint(), r.Uint(); nodes == 0 {
			t.Fatal("no nodes")
		}
	}
	if r.Err() != nil || r.Len() != 0 {
		t.Fatalf("result: err %v, %d bytes left", r.Err(), r.Len())
	}

	file := func(i uint64) decoded {
		var w wire.Writer
		w.Int(h)
		w.Uint(i)
		resp, err := kcabi.Call(OpLintFile, w.Message())
		if err != nil {
			t.Fatal(err)
		}
		return decode(t, resp)
	}
	da := file(0)
	var call *dcall
	for _, f := range da.facts {
		if f.call != nil && f.call.name == "p.g" {
			call = f.call
		}
	}
	if call == nil {
		t.Fatalf("no call of p.g in %+v", da.facts)
	}
	if call.decl[0] != 2 || len(call.params) != 2 || call.params[1] != "b" || len(call.mapping) != 2 || call.mapping[1] != 1 {
		t.Fatalf("call %+v", call)
	}
	if call.ret == 0 || da.types[call.ret-1].name != "kotlin.Int" {
		t.Fatalf("call returns %d", call.ret)
	}
	db := file(1)
	var ret bool
	for _, f := range db.facts {
		if f.ret[0] == 2 {
			ret = true
		}
	}
	if !ret {
		t.Fatal("no return target in b.kt")
	}

	w = wire.Writer{}
	w.Int(h)
	if _, err := kcabi.Call(OpLintClose, w.Message()); err != nil {
		t.Fatal(err)
	}
	w = wire.Writer{}
	w.Int(h)
	w.Uint(0)
	if _, err := kcabi.Call(OpLintFile, w.Message()); err == nil {
		t.Fatal("file of a closed result")
	}
}
