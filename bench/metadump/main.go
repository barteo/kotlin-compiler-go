// Command metadump prints the Kotlin metadata of a directory of class
// files as text, one declaration per line, sorted, for comparing two
// builds with diff:
//
//	go run ./bench/metadump kotlinc-classes/ > a.txt
//	go run ./bench/metadump kc-classes/ > b.txt
//	diff a.txt b.txt
//
// Type parameters are printed by name, so builds that number them
// differently compare equal. A class file without metadata prints a line
// that says so.
package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: metadump classes-dir")
		os.Exit(2)
	}
	var lines []string
	err := filepath.WalkDir(os.Args[1], func(path string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".class") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := classfile.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		ls, err := dump(info)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		lines = append(lines, ls...)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Println(l)
	}
}

// printer formats declarations, with the type parameters in scope.
type printer struct{ scope []*kmeta.TypeParam }

func (p printer) with(ps []*kmeta.TypeParam) printer {
	return printer{append(append([]*kmeta.TypeParam(nil), p.scope...), ps...)}
}

func dump(info *classfile.Info) ([]string, error) {
	m := info.Kotlin
	if m == nil {
		return []string{info.Name + " no metadata"}, nil
	}
	prefix := fmt.Sprintf("%s k=%d", info.Name, m.K)
	switch m.K {
	case kmeta.KindClassFile:
		c, err := kmeta.ReadClass(m)
		if err != nil {
			return nil, err
		}
		return dumpClass(prefix, c), nil
	case kmeta.KindFileFacade, kmeta.KindMultiFilePart:
		pkg, err := kmeta.ReadPackage(m)
		if err != nil {
			return nil, err
		}
		d, err := pkg.All()
		if err != nil {
			return nil, err
		}
		out := []string{prefix + " package module=" + d.ModuleName}
		var p printer
		for _, f := range d.Funcs {
			out = append(out, prefix+" "+p.function(f))
		}
		for _, pr := range d.Props {
			out = append(out, prefix+" "+p.property(pr))
		}
		for _, a := range d.Aliases {
			out = append(out, prefix+" "+p.alias(a))
		}
		return out, nil
	case kmeta.KindSynthetic:
		prefix += fmt.Sprintf(" xi=%d", m.XI)
		f, err := kmeta.ReadLambda(m)
		if err != nil || f == nil {
			return []string{prefix}, err
		}
		return []string{prefix + " lambda " + printer{}.function(f)}, nil
	}
	return []string{prefix}, nil
}

func dumpClass(prefix string, c *kmeta.Class) []string {
	p := printer{}.with(c.TypeParams)
	head := fmt.Sprintf("%s class %s %s name=%s companion=%s module=%s jvmflags=%d", prefix, classFlags(c), p.typeParams(c.TypeParams), c.Name, c.Companion, c.ModuleName, c.JVMFlags)
	out := []string{head}
	add := func(s string) { out = append(out, prefix+" "+s) }
	var supers []string
	for _, t := range c.Supers {
		supers = append(supers, p.typ(t))
	}
	add("supers " + strings.Join(supers, ", "))
	add("nested " + strings.Join(c.Nested, ", "))
	if len(c.EnumEntries) > 0 {
		add("entries " + strings.Join(c.EnumEntries, ", "))
	}
	if len(c.SealedSubclasses) > 0 {
		add("sealed " + strings.Join(c.SealedSubclasses, ", "))
	}
	if c.Underlying != nil {
		add("underlying " + c.UnderlyingProp + ": " + p.typ(c.Underlying))
	}
	for _, k := range c.Ctors {
		add(fmt.Sprintf("constructor %s(%s) jvm=%s%s", flags(k.Flags, "ctor"), p.params(k.Params), k.JVM.Name, k.JVM.Desc))
	}
	for _, f := range c.Funcs {
		add(p.function(f))
	}
	for _, pr := range c.Props {
		add(p.property(pr))
	}
	for _, a := range c.Aliases {
		add(p.alias(a))
	}
	return out
}

func (p printer) alias(a *kmeta.TypeAlias) string {
	q := p.with(a.TypeParams)
	typ := func(t *kmeta.Type) string {
		if t == nil {
			return "-"
		}
		return q.typ(t)
	}
	return fmt.Sprintf("typealias %s %s%s = %s expanded=%s", flags(a.Flags, "alias"), a.Name, q.typeParams(a.TypeParams), typ(a.Underlying), typ(a.Expanded))
}

func (p printer) function(f *kmeta.Function) string {
	q := p.with(f.TypeParams)
	ctx := ""
	if len(f.ContextParams) > 0 {
		ctx = "context(" + q.params(f.ContextParams) + ") "
	} else if f.Contexts > 0 {
		ctx = "context(" + strconv.Itoa(f.Contexts) + " receivers) "
	}
	recv := ""
	if f.Recv != nil {
		recv = q.typ(f.Recv) + "."
	}
	return fmt.Sprintf("fun %s %s%s%s%s(%s): %s jvm=%s%s", flags(f.Flags, "fun"), ctx, q.typeParams(f.TypeParams), recv, f.Name,
		q.params(f.Params), q.typ(f.Ret), f.JVM.Name, f.JVM.Desc)
}

func (p printer) property(pr *kmeta.Property) string {
	q := p.with(pr.TypeParams)
	recv := ""
	if pr.Recv != nil {
		recv = q.typ(pr.Recv) + "."
	}
	sig := func(name string, s *kmeta.JVMSig) string {
		if s == nil {
			return ""
		}
		return " " + name + "=" + s.Name + s.Desc
	}
	setterParam := ""
	if pr.SetterParam != nil {
		setterParam = " setparam=" + q.params([]*kmeta.Param{pr.SetterParam})
	}
	return fmt.Sprintf("prop %s %s%s%s: %s get=%s set=%s%s%s%s%s%s", flags(pr.Flags, "prop"), q.typeParams(pr.TypeParams), recv, pr.Name,
		q.typ(pr.Type), flags(pr.GetterFlags, "accessor"), flags(pr.SetterFlags, "accessor"), setterParam,
		sig("field", pr.Field), sig("getter", pr.Getter), sig("setter", pr.Setter), sig("annotations", pr.Synthetic))
}

func (p printer) params(ps []*kmeta.Param) string {
	var out []string
	for _, v := range ps {
		s := v.Name + ": " + p.typ(v.Type)
		if v.Vararg != nil {
			s = "vararg " + s + " of " + p.typ(v.Vararg)
		}
		var fs []string
		for _, f := range []struct {
			bit  int
			name string
		}{{1, "annotations"}, {kmeta.ParamDeclaresDefault, "default"}, {kmeta.ParamCrossinline, "crossinline"}, {kmeta.ParamNoinline, "noinline"}} {
			if v.Flags&f.bit != 0 {
				fs = append(fs, f.name)
			}
		}
		if len(fs) > 0 {
			s += " [" + strings.Join(fs, " ") + "]"
		}
		out = append(out, s)
	}
	return strings.Join(out, ", ")
}

func (p printer) typeParams(ps []*kmeta.TypeParam) string {
	if len(ps) == 0 {
		return ""
	}
	var out []string
	for _, tp := range ps {
		s := tp.Name
		switch tp.Variance {
		case kmeta.In:
			s = "in " + s
		case kmeta.Out:
			s = "out " + s
		}
		if tp.Reified {
			s = "reified " + s
		}
		var bs []string
		for _, b := range tp.Bounds {
			bs = append(bs, p.typ(b))
		}
		if len(bs) > 0 {
			s += " : " + strings.Join(bs, " & ")
		}
		out = append(out, s)
	}
	return "<" + strings.Join(out, ", ") + ">"
}

func (p printer) typ(t *kmeta.Type) string {
	if t == nil {
		return "?nil"
	}
	var s string
	switch {
	case t.Class != "":
		s = t.Class
	case t.ParamName != "":
		s = t.ParamName
	default:
		s = fmt.Sprintf("#%d", t.Param)
		for _, tp := range p.scope {
			if tp.ID == t.Param {
				s = tp.Name
			}
		}
	}
	if len(t.Args) > 0 {
		var as []string
		for _, a := range t.Args {
			switch a.Proj {
			case kmeta.Star:
				as = append(as, "*")
			case kmeta.In:
				as = append(as, "in "+p.typ(a.Type))
			case kmeta.Out:
				as = append(as, "out "+p.typ(a.Type))
			default:
				as = append(as, p.typ(a.Type))
			}
		}
		s += "<" + strings.Join(as, ", ") + ">"
	}
	if t.Outer != nil {
		s = p.typ(t.Outer) + "." + s
	}
	if t.Nullable {
		s += "?"
	}
	if t.Flags&kmeta.TypeDefinitelyNonNull != 0 {
		s += " & Any"
	}
	if t.ExtFunc {
		s = "@ExtensionFunctionType " + s
	}
	if t.Contexts > 0 {
		s = "@ContextFunctionTypeParams(" + strconv.Itoa(t.Contexts) + ") " + s
	}
	if t.Flags&kmeta.TypeSuspend != 0 {
		s = "suspend " + s
	}
	if t.Upper != nil {
		s = "(" + s + ".." + p.typ(t.Upper) + ")"
	}
	return s
}

var visibilities = []string{"internal", "private", "protected", "public", "private_to_this", "local"}
var modalities = []string{"final", "open", "abstract", "sealed"}
var memberKinds = []string{"declaration", "fake_override", "delegation", "synthesized"}

// flags names the bits of a declaration's flags.
func flags(f kmeta.Flags, kind string) string {
	var out []string
	if f&1 != 0 {
		out = append(out, "annotations")
	}
	out = append(out, visibilities[f.Visibility()])
	if kind != "ctor" {
		out = append(out, modalities[f.Modality()])
	}
	bit := func(n int, name string) {
		if f>>n&1 != 0 {
			out = append(out, name)
		}
	}
	switch kind {
	case "fun":
		out = append(out, memberKinds[f.MemberKind()])
		for i, n := range []string{"operator", "infix", "inline", "tailrec", "external", "suspend", "expect", "non_stable_names"} {
			bit(8+i, n)
		}
	case "prop":
		out = append(out, memberKinds[f.MemberKind()])
		for i, n := range []string{"var", "getter", "setter", "const", "lateinit", "constant", "external", "delegated", "expect"} {
			bit(8+i, n)
		}
	case "accessor":
		for i, n := range []string{"not_default", "external", "inline"} {
			bit(6+i, n)
		}
	case "ctor":
		for i, n := range []string{"secondary", "non_stable_names"} {
			bit(4+i, n)
		}
	}
	return "[" + strings.Join(out, " ") + "]"
}

var classKinds = []string{"class", "interface", "enum_class", "enum_entry", "annotation_class", "object", "companion_object"}

func classFlags(c *kmeta.Class) string {
	f := c.Flags
	var out []string
	if f&1 != 0 {
		out = append(out, "annotations")
	}
	out = append(out, visibilities[f.Visibility()], modalities[f.Modality()], classKinds[c.Kind()])
	for i, n := range []string{"inner", "data", "external", "expect", "value", "fun", "enum_entries"} {
		if f>>(9+i)&1 != 0 {
			out = append(out, n)
		}
	}
	return "[" + strings.Join(out, " ") + "]"
}
