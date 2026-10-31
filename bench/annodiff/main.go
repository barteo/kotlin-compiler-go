// Command annodiff compares the annotations of two directories of class
// files: of each class, field, method and parameter, with their values.
//
//	go run ./bench/annodiff kotlinc-classes/ kc-classes/
//
// It prints a line for each annotation that only one directory has, "-"
// for the first and "+" for the second, and exits with status 1 if there
// are any. Members that only one directory has are left out: bench/apidump
// compares those. @kotlin.Metadata and kotlinc's SourceDebugExtension
// aren't compared, and with -nullability neither are
// org.jetbrains.annotations.NotNull and Nullable.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	noNull := flag.Bool("nullability", false, "leave out NotNull and Nullable")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: annodiff [-nullability] dir1 dir2")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	a, err := dump(flag.Arg(0), *noNull)
	if err != nil {
		log.Fatal(err)
	}
	b, err := dump(flag.Arg(1), *noNull)
	if err != nil {
		log.Fatal(err)
	}
	var diffs []string
	for key, as := range a {
		bs, ok := b[key]
		if !ok {
			continue
		}
		for _, l := range minus(as, bs) {
			diffs = append(diffs, "- "+key+" "+l)
		}
		for _, l := range minus(bs, as) {
			diffs = append(diffs, "+ "+key+" "+l)
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i][2:] < diffs[j][2:] || diffs[i][2:] == diffs[j][2:] && diffs[i] < diffs[j]
	})
	for _, d := range diffs {
		fmt.Println(d)
	}
	fmt.Printf("%d differences in %d declarations compared\n", len(diffs), len(a))
	if len(diffs) > 0 {
		os.Exit(1)
	}
}

// minus returns the elements of a that b lacks, counting duplicates.
func minus(a, b []string) []string {
	n := map[string]int{}
	for _, s := range b {
		n[s]++
	}
	var out []string
	for _, s := range a {
		if n[s] > 0 {
			n[s]--
			continue
		}
		out = append(out, s)
	}
	return out
}

// dump returns the annotations of the declarations of the class files in
// dir, by declaration: "Class", "Class.name desc" or "Class.name desc#i"
// for a parameter.
func dump(dir string, noNull bool) (map[string][]string, error) {
	out := map[string][]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".class") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := parse(data, out, noNull); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	})
	return out, err
}

type reader struct {
	b   []byte
	pos int
}

func (r *reader) u1() int { v := r.b[r.pos]; r.pos++; return int(v) }
func (r *reader) u2() int { v := binary.BigEndian.Uint16(r.b[r.pos:]); r.pos += 2; return int(v) }
func (r *reader) u4() int { v := binary.BigEndian.Uint32(r.b[r.pos:]); r.pos += 4; return int(v) }

type entry struct {
	tag  int
	str  string // Utf8; the name of a Class or String
	num  uint64
	ref  int
	ref2 int
}

type class struct {
	pool   []entry
	noNull bool
}

func (c *class) utf8(i int) string { return c.pool[i].str }

func parse(data []byte, out map[string][]string, noNull bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("malformed class file: %v", r)
		}
	}()
	r := &reader{b: data, pos: 8}
	c := &class{noNull: noNull}
	n := r.u2()
	c.pool = make([]entry, n)
	for i := 1; i < n; i++ {
		e := entry{tag: r.u1()}
		switch e.tag {
		case 1:
			l := r.u2()
			e.str = string(r.b[r.pos : r.pos+l])
			r.pos += l
		case 3, 4:
			e.num = uint64(r.u4())
		case 5, 6:
			e.num = uint64(r.u4())<<32 | uint64(r.u4())
		case 7, 8, 16, 19, 20:
			e.ref = r.u2()
		case 9, 10, 11, 12, 18, 17:
			e.ref, e.ref2 = r.u2(), r.u2()
		case 15:
			r.u1()
			e.ref = r.u2()
		default:
			return fmt.Errorf("constant tag %d", e.tag)
		}
		c.pool[i] = e
		if e.tag == 5 || e.tag == 6 {
			i++
		}
	}
	r.u2() // access
	name := c.utf8(c.pool[r.u2()].ref)
	r.u2() // super
	r.pos += 2 * r.u2()
	for range 2 { // fields, then methods
		for range r.u2() {
			r.u2()
			key := name + "." + c.utf8(r.u2()) + " " + c.utf8(r.u2())
			c.attrs(r, key, out)
		}
	}
	c.attrs(r, name, out)
	return nil
}

// attrs reads attributes, recording the annotations among them for key.
func (c *class) attrs(r *reader, key string, out map[string][]string) {
	for range r.u2() {
		aname := c.utf8(r.u2())
		size := r.u4()
		end := r.pos + size
		switch aname {
		case "RuntimeVisibleAnnotations", "RuntimeInvisibleAnnotations":
			vis := aname == "RuntimeVisibleAnnotations"
			for range r.u2() {
				if s, ok := c.annotation(r, vis); ok {
					out[key] = append(out[key], s)
				}
			}
		case "RuntimeVisibleParameterAnnotations", "RuntimeInvisibleParameterAnnotations":
			vis := aname == "RuntimeVisibleParameterAnnotations"
			for i := range r.u1() {
				for range r.u2() {
					if s, ok := c.annotation(r, vis); ok {
						k := fmt.Sprintf("%s#%d", key, i)
						out[k] = append(out[k], s)
					}
				}
			}
		case "AnnotationDefault":
			out[key] = append(out[key], "default="+c.value(r))
		case "Deprecated":
			out[key] = append(out[key], "Deprecated")
		}
		if _, ok := out[key]; !ok {
			out[key] = nil // compared even without annotations
		}
		r.pos = end
	}
	if _, ok := out[key]; !ok {
		out[key] = nil
	}
}

func (c *class) annotation(r *reader, visible bool) (string, bool) {
	typ := c.utf8(r.u2())
	var vals []string
	for range r.u2() {
		vals = append(vals, c.utf8(r.u2())+"="+c.value(r))
	}
	switch typ {
	case "Lkotlin/Metadata;", "Lkotlin/jvm/internal/SourceDebugExtension;":
		return "", false
	case "Lorg/jetbrains/annotations/NotNull;", "Lorg/jetbrains/annotations/Nullable;":
		if c.noNull {
			return "", false
		}
	}
	v := "invisible"
	if visible {
		v = "visible"
	}
	return fmt.Sprintf("%s %s(%s)", v, typ, strings.Join(vals, ", ")), true
}

func (c *class) value(r *reader) string {
	switch tag := r.u1(); tag {
	case 'B', 'C', 'I', 'S', 'Z':
		return fmt.Sprintf("%c%d", tag, int32(c.pool[r.u2()].num))
	case 'J':
		return fmt.Sprintf("J%d", int64(c.pool[r.u2()].num))
	case 'F':
		return fmt.Sprintf("F%v", math.Float32frombits(uint32(c.pool[r.u2()].num)))
	case 'D':
		return fmt.Sprintf("D%v", math.Float64frombits(c.pool[r.u2()].num))
	case 's':
		return fmt.Sprintf("%q", c.utf8(r.u2()))
	case 'e':
		t := c.utf8(r.u2())
		return t + "." + c.utf8(r.u2())
	case 'c':
		return "class " + c.utf8(r.u2())
	case '@':
		s, _ := c.annotation(r, true)
		return "@" + strings.TrimPrefix(s, "visible ")
	case '[':
		var es []string
		for range r.u2() {
			es = append(es, c.value(r))
		}
		return "[" + strings.Join(es, ", ") + "]"
	default:
		panic(fmt.Sprintf("element value tag %c", tag))
	}
}
