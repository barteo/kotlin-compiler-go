// Command apidump prints the public API of a directory of class files in
// the format of Kotlin's binary-compatibility-validator (.api files), and
// compares it with a reference dump.
//
//	go run ./bench/apidump out/                       # print the dump
//	go run ./bench/apidump -api kotlinpoet.api out/   # compare
//
// The comparison lists the classes and members of the reference that the
// directory lacks, or declares differently, and exits with status 1 if
// there are any: a caller compiled against the reference may fail to link
// against them. Members the directory has in addition are listed for
// information. The validator also reads Kotlin metadata to leave out
// internal declarations, which are public in the class file; apidump
// doesn't, so internal declarations are among the extras.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
)

func main() {
	api := flag.String("api", "", "reference .api file to compare with")
	out := flag.String("o", "", "also write the dump to this file")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: apidump [-api file.api] [-o dump.api] classes-dir")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	classes, err := dump(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	text := format(classes)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	if *api == "" {
		fmt.Print(text)
		return
	}
	data, err := os.ReadFile(*api)
	if err != nil {
		log.Fatal(err)
	}
	if !compare(os.Stdout, parse(string(data)), classes) {
		os.Exit(1)
	}
}

// class is a class of a dump: its header line and its members' lines.
type class struct {
	name    string
	header  string
	members []string
}

// accessNames are the access flags a dump shows, in its order.
var accessNames = []struct {
	flag uint16
	name string
}{
	{cf.AccPublic, "public"},
	{cf.AccProtected, "protected"},
	{cf.AccPrivate, "private"},
	{cf.AccStatic, "static"},
	{cf.AccFinal, "final"},
	{cf.AccAbstract, "abstract"},
	{cf.AccSynthetic, "synthetic"},
	{cf.AccInterface, "interface"},
	{cf.AccAnnotation, "annotation"},
}

func modifiers(access uint16) string {
	var ws []string
	for _, a := range accessNames {
		if access&a.flag != 0 {
			ws = append(ws, a.name)
		}
	}
	return strings.Join(ws, " ")
}

// dump reads the class files under dir and returns their public API,
// sorted by class name.
func dump(dir string) ([]*class, error) {
	infos := map[string]*cf.Info{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".class") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := cf.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %v", p, err)
		}
		infos[info.Name] = info
		return nil
	})
	if err != nil {
		return nil, err
	}
	var classes []*class
	for _, info := range infos {
		if !public(info, infos) {
			continue
		}
		classes = append(classes, apiOf(info))
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].name < classes[j].name })
	return classes, nil
}

// public reports whether a class is part of the public API: public or
// protected, not a synthetic class of Kotlin's, and in a public class if
// it is nested.
func public(info *cf.Info, infos map[string]*cf.Info) bool {
	// Kotlin's synthetic classes are lambdas, when mappings and the
	// like, but for the DefaultImpls of interfaces. As with the
	// validator, their metadata tells them, not ACC_SYNTHETIC, which
	// serialization's $serializer classes have too.
	if info.Kotlin != nil && info.Kotlin.K == 3 && !strings.HasSuffix(info.Name, "$DefaultImpls") {
		return false
	}
	if info.Access&cf.AccSynthetic != 0 && info.Kotlin == nil {
		return false
	}
	access, outer := info.Access, ""
	if ic := info.Inner; ic != nil {
		if ic.Outer == "" {
			return false // local or anonymous
		}
		access, outer = ic.Access, ic.Outer
	} else if i := strings.LastIndexByte(info.Name, '$'); i > 0 {
		// Without an InnerClasses attribute, the name tells the outer
		// class, if the directory has it.
		if _, ok := infos[info.Name[:i]]; ok {
			outer = info.Name[:i]
		}
	}
	if access&(cf.AccPublic|cf.AccProtected) == 0 {
		return false
	}
	if outer != "" {
		o, ok := infos[outer]
		return ok && public(o, infos)
	}
	return true
}

// apiOf returns the dump of a public class: its fields by name, then its
// methods by name and descriptor.
func apiOf(info *cf.Info) *class {
	c := &class{name: info.Name}
	var supers []string
	if info.Super != "" && info.Super != "java/lang/Object" {
		supers = append(supers, info.Super)
	}
	interfaces := append([]string(nil), info.Interfaces...)
	sort.Strings(interfaces)
	supers = append(supers, interfaces...)
	// The class file's own flags, without ACC_SUPER, which is ACC_SYNCHRONIZED's bit.
	c.header = modifiers(info.Access&^cf.AccSuper) + " class " + info.Name
	if len(supers) > 0 {
		c.header += " : " + strings.Join(supers, ", ")
	}
	final := info.Access&cf.AccFinal != 0
	visible := func(m cf.Member) bool {
		switch {
		case m.Access&cf.AccPublic != 0:
		case m.Access&cf.AccProtected != 0 && !final:
		default:
			return false
		}
		// Synthetic accessors of private members.
		return m.Access&cf.AccSynthetic == 0 || !strings.HasPrefix(m.Name, "access$") && !strings.HasSuffix(m.Name, "$annotations")
	}
	fields := append([]cf.Member(nil), info.Fields...)
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	for _, f := range fields {
		if visible(f) {
			// A field's ACC_VOLATILE and ACC_TRANSIENT share bits with
			// ACC_BRIDGE and ACC_VARARGS, which modifiers doesn't show.
			c.members = append(c.members, modifiers(f.Access)+" field "+f.Name+" "+f.Desc)
		}
	}
	methods := append([]cf.Member(nil), info.Methods...)
	sort.SliceStable(methods, func(i, j int) bool {
		if methods[i].Name != methods[j].Name {
			return methods[i].Name < methods[j].Name
		}
		return methods[i].Desc < methods[j].Desc
	})
	for _, m := range methods {
		if visible(m) {
			c.members = append(c.members, modifiers(m.Access)+" fun "+m.Name+" "+m.Desc)
		}
	}
	return c
}

func format(classes []*class) string {
	var sb strings.Builder
	for i, c := range classes {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "%s {\n", c.header)
		for _, m := range c.members {
			fmt.Fprintf(&sb, "\t%s\n", m)
		}
		sb.WriteString("}\n")
	}
	return sb.String()
}

// parse reads a .api file.
func parse(text string) []*class {
	var classes []*class
	var c *class
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasSuffix(line, " {") && !strings.HasPrefix(line, "\t"):
			header := strings.TrimSuffix(line, " {")
			c = &class{header: header, name: className(header)}
			classes = append(classes, c)
		case line == "}":
			c = nil
		case c != nil && strings.HasPrefix(line, "\t"):
			c.members = append(c.members, strings.TrimPrefix(line, "\t"))
		}
	}
	return classes
}

// className returns the class name of a header line.
func className(header string) string {
	head, _, _ := strings.Cut(header, " : ")
	return head[strings.LastIndexByte(head, ' ')+1:]
}

// memberKey identifies a member: its name and descriptor, with the
// modifiers left out.
func memberKey(m string) string {
	ws := strings.Fields(m)
	if len(ws) < 3 {
		return m
	}
	return strings.Join(ws[len(ws)-3:], " ")
}

// compare writes the differences between the reference API want and got,
// and reports whether got has everything want has, as want declares it.
func compare(w *os.File, want, got []*class) bool {
	gotBy := map[string]*class{}
	for _, c := range got {
		gotBy[c.name] = c
	}
	wantBy := map[string]bool{}
	var missingClasses, missingMembers, changed int
	var report strings.Builder
	for _, wc := range want {
		wantBy[wc.name] = true
		gc := gotBy[wc.name]
		if gc == nil {
			missingClasses++
			fmt.Fprintf(&report, "missing class %s (%d members)\n", wc.name, len(wc.members))
			continue
		}
		var lines []string
		if gc.header != wc.header {
			changed++
			lines = append(lines, "  - "+wc.header, "  + "+gc.header)
		}
		gotMembers := map[string]string{}
		for _, m := range gc.members {
			gotMembers[memberKey(m)] = m
		}
		wantMembers := map[string]bool{}
		for _, m := range wc.members {
			k := memberKey(m)
			wantMembers[k] = true
			switch g, ok := gotMembers[k]; {
			case !ok:
				missingMembers++
				lines = append(lines, "  missing: "+m)
			case g != m:
				changed++
				lines = append(lines, "  - "+m, "  + "+g)
			}
		}
		for _, m := range gc.members {
			if !wantMembers[memberKey(m)] {
				lines = append(lines, "  extra: "+m)
			}
		}
		if len(lines) > 0 {
			fmt.Fprintf(&report, "%s\n%s\n", wc.name, strings.Join(lines, "\n"))
		}
	}
	extraClasses := 0
	for _, gc := range got {
		if !wantBy[gc.name] {
			extraClasses++
			fmt.Fprintf(&report, "extra class %s (%d members)\n", gc.name, len(gc.members))
		}
	}
	fmt.Fprint(w, report.String())
	fmt.Fprintf(w, "\n%d missing classes, %d missing members, %d declared differently; %d extra classes\n",
		missingClasses, missingMembers, changed, extraClasses)
	return missingClasses == 0 && missingMembers == 0 && changed == 0
}
