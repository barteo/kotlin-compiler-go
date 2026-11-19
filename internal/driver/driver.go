// Package driver runs the compilation pipeline.
package driver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/codegen"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type Options struct {
	// Frontend picks the parser: "syntax", kc's token parser, the
	// default, or "psi", which parses a lossless psi tree and lowers it.
	// If empty, $KC_FRONTEND picks it.
	Frontend  string
	OutDir    string
	Jobs      int
	Timing    io.Writer // if set, per-phase timings are written here
	ClassPath []string  // jars and directories, searched after the JDK
	// FriendPaths are the entries of ClassPath whose internal declarations
	// the module sees, as with kotlinc's -Xfriend-paths: a unit test's main
	// code.
	FriendPaths []string
	// KeepHidden keeps Java classes' members that Kotlin can't use, for
	// the Analysis API: see check.Library.KeepHidden.
	KeepHidden bool
	// JavaTypes reads Java declarations' types as kotlinc does where
	// checking approximates them, for the Analysis API: see
	// check.Library.JavaTypes.
	JavaTypes bool
	JDKHome   string // JDK whose classes are used; found automatically if empty
	NoJDK     bool   // don't use JDK classes
	// ModuleName is the module's name, which the JVM names of internal
	// members end with, as with kotlinc's -module-name; "main" if empty.
	ModuleName string
	// NoJVMDefault compiles interfaces without JVM default methods, their
	// bodies in DefaultImpls classes, as kotlinc's -jvm-default=disable.
	NoJVMDefault bool
	// JVMDefaultCompat compiles interfaces with JVM default methods and
	// their bodies also in DefaultImpls classes, and classes with bridges
	// to the default methods they inherit, as kotlinc's -jvm-default=enable.
	JVMDefaultCompat bool
	// Parcelize compiles as kotlinc does with the parcelize compiler
	// plugin: classes annotated @Parcelize get their Parcelable members.
	Parcelize bool
	// Serialization compiles as kotlinc does with the kotlinx.serialization
	// compiler plugin: classes annotated @Serializable get their
	// serializers.
	Serialization bool
	// Compose compiles as kotlinc does with the Compose compiler plugin:
	// @Composable functions take a Composer and change flags.
	Compose bool
	// Metro compiles as kotlinc does with the Metro dependency injection
	// compiler plugin: @DependencyGraph interfaces get implementations
	// that build what they return.
	Metro bool
	// MetroOptions are the Metro plugin's options, by key, as kotlinc's
	// -P plugin:dev.zacsweers.metro.compiler:key=value passes them.
	MetroOptions map[string]string
	// Debug compiles as kotlinc's -Xdebug: suspend functions save dead
	// variables' values in their continuations as they are, not nulled
	// out, for debuggers.
	Debug bool
	// OptIns are the qualified names of the requirement markers the whole
	// module opts in to, as kotlinc's -opt-in.
	OptIns []string
	// ExtraWarnings also reports the warnings kotlinc reports with
	// -Wextra, and ExperimentalWarnings those it reports with
	// -Xuse-fir-experimental-checkers: see check.Options.
	ExtraWarnings, ExperimentalWarnings bool
	// Libraries, if set, keeps the class path and library for later
	// compiles of the same configuration, and gives this compile those
	// of an earlier one: see Libraries.
	Libraries *Libraries
}

// parallel runs fn(i) for i in [0, n) on up to jobs goroutines.
func parallel(n, jobs int, fn func(i int)) {
	if jobs < 1 {
		jobs = 1
	}
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(jobs, n) {
		wg.Go(func() {
			for i := range next {
				fn(i)
			}
		})
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

// OpenLibrary opens the class path opts names (the JDK, unless NoJDK, and
// ClassPath and FriendPaths) and returns the library the checker reads it
// through, set up as opts says. The caller closes the class path when it
// is done with the library. Java sources are not read.
func OpenLibrary(opts Options) (*classpath.Path, *check.Library, error) {
	cp, err := openClassPath(opts)
	if err != nil {
		return nil, nil, err
	}
	return cp, newLibrary(cp, opts), nil
}

// openClassPath opens the class path opts names.
func openClassPath(opts Options) (*classpath.Path, error) {
	jdk := opts.JDKHome
	if jdk == "" && !opts.NoJDK {
		// Without a JDK, programs can use everything but Java classes.
		jdk, _ = classpath.FindJDK()
	}
	return classpath.Open(jdk, opts.ClassPath, opts.FriendPaths...)
}

// newLibrary returns the library over cp, set up as opts says.
func newLibrary(cp *classpath.Path, opts Options) *check.Library {
	lib := check.NewLibrary(cp)
	lib.Module = opts.ModuleName
	lib.NoJVMDefault = opts.NoJVMDefault
	lib.JVMDefaultCompat = opts.JVMDefaultCompat
	lib.Parcelize = opts.Parcelize
	lib.Compose = opts.Compose
	lib.OptIns = opts.OptIns
	if opts.KeepHidden {
		lib.KeepHidden()
	}
	if opts.JavaTypes {
		lib.JavaTypes()
	}
	if opts.Serialization {
		lib.EnableSerialization()
	}
	return lib
}

// Compile compiles the given .kt files into class files in opts.OutDir.
// The .java files among them are the module's Java sources, whose
// declarations the Kotlin code sees, as kotlinc's; javac compiles them.
// Diagnostics are returned as a joined error.
func Compile(paths []string, opts Options) error {
	_, err := CompileWarnings(paths, opts)
	return err
}

// CompileWarnings is Compile, which also returns the warnings, whether
// or not the compile fails: those of the files checked, if they were.
func CompileWarnings(paths []string, opts Options) (warnings []*syntax.Error, err error) {
	err = compile(paths, opts, &warnings)
	return warnings, err
}

func compile(paths []string, opts Options, warnings *[]*syntax.Error) error {
	var javaPaths []string
	paths = slices.DeleteFunc(slices.Clone(paths), func(p string) bool {
		if strings.HasSuffix(p, ".java") {
			javaPaths = append(javaPaths, p)
			return true
		}
		return false
	})
	start := time.Now()
	phase := func(name string) {
		if opts.Timing != nil {
			now := time.Now()
			fmt.Fprintf(opts.Timing, "%-8s %v\n", name, now.Sub(start).Round(time.Microsecond))
			start = now
		}
	}

	// The class path is indexed while the sources are parsed. Then the
	// Kotlin standard library's default imports are read, also in the
	// background: checking waits for a package only if it is not read yet.
	type cpResult struct {
		cp     *classpath.Path
		lib    *check.Library
		done   func() // called once the compile is done with cp
		shared bool   // cp and lib's classes are opts.Libraries': see Libraries
		err    error
	}
	cpDone := make(chan cpResult, 1)
	// The classes the Java sources name that aren't theirs. The program's
	// Kotlin classes are added once they are parsed, before checking reads
	// any Java source's class.
	java := &javaOutside{}
	var prefetch time.Duration
	prefetched := make(chan struct{})
	go func() {
		var r cpResult
		if opts.Libraries != nil && len(javaPaths) == 0 {
			// A shared library's default imports are read once: then
			// Prefetch finds them read.
			r.cp, r.lib, r.done, r.shared, r.err = opts.Libraries.get(opts)
		} else {
			r.cp, r.err = openClassPath(opts)
			if r.err == nil && len(javaPaths) > 0 {
				r.err = addJava(r.cp, javaPaths, opts.Jobs, java)
			}
			if r.err == nil {
				r.lib = newLibrary(r.cp, opts)
				r.done = func() { r.cp.Close() }
			}
		}
		lib := r.lib
		cpDone <- r
		if lib != nil {
			start := time.Now()
			lib.Prefetch()
			prefetch = time.Since(start)
		}
		close(prefetched)
	}()
	openCP := sync.OnceValue(func() cpResult { return <-cpDone })
	defer func() {
		if r := openCP(); r.err == nil {
			<-prefetched
			r.done()
		}
	}()

	frontend, err := frontendOf(opts)
	if err != nil {
		return err
	}
	files := make([]*syntax.File, len(paths))
	srcs := make([]string, len(paths))
	errLists := make([][]*syntax.Error, len(paths))
	softLists := make([][]*syntax.Error, len(paths))
	ioErrs := make([]error, len(paths))
	parallel(len(paths), opts.Jobs, func(i int) {
		src, err := os.ReadFile(paths[i])
		if err != nil {
			ioErrs[i] = err
			return
		}
		srcs[i] = string(src)
		files[i], errLists[i], softLists[i] = parse(frontend, paths[i], srcs[i])
	})
	if err := errors.Join(ioErrs...); err != nil {
		return err
	}
	// After syntax errors, the files are checked as far as the parser
	// recovered them, as kotlinc does, for the errors it finds besides
	// (see checkRecovered); a file the lexer rejects isn't.
	var diags []error
	broken, unrecovered := false, false
	for i, es := range errLists {
		for _, e := range es {
			diags = append(diags, e)
		}
		broken = broken || len(es)+len(softLists[i]) > 0
		unrecovered = unrecovered || len(es)+len(softLists[i]) > 0 && files[i] == nil
	}
	// Until the checker runs, kc's parser's errors are all reported.
	early := slices.Clip(diags)
	for _, es := range softLists {
		for _, e := range es {
			early = append(early, e)
		}
	}
	if unrecovered {
		return errors.Join(early...)
	}
	classes := map[string]string{} // the files' facades and parts, by name
	multifile := map[string]bool{} // the multi-file classes' facades
	for _, f := range files {
		if !f.HasFacade() {
			continue // a file of expect declarations, as a common one
		}
		if prev, dup := classes[f.Class()]; dup && broken {
			return errors.Join(early...) // as when only the syntax was checked
		} else if dup {
			return fmt.Errorf("%s: duplicate JVM class name %s (also from %s)", f.Path, f.Class(), prev)
		}
		classes[f.Class()] = f.Path
		if f.Part != "" {
			multifile[f.Facade] = true
		}
	}
	for name := range multifile {
		if prev, dup := classes[name]; dup && broken {
			return errors.Join(early...)
		} else if dup {
			return fmt.Errorf("%s: duplicate JVM class name %s (also a multi-file class)", prev, name)
		}
	}
	phase("parse")
	r := openCP()
	if r.err != nil && broken {
		return errors.Join(early...) // as without the class path
	}
	if r.err != nil {
		return r.err
	}
	cp := r.cp
	java.cp = cp
	if len(javaPaths) > 0 {
		java.kotlin = kotlinClasses(files)
	}
	if opts.Timing != nil && r.shared {
		fmt.Fprintf(opts.Timing, "%-8s 0s (%d classes, indexed by an earlier compile; waited %v after parse)\n", "index",
			cp.Len(), time.Since(start).Round(time.Microsecond))
	} else if opts.Timing != nil {
		fmt.Fprintf(opts.Timing, "%-8s %v (%d classes, concurrent with parse; waited %v after it)\n", "index",
			cp.IndexTime.Round(time.Microsecond), cp.Len(), time.Since(start).Round(time.Microsecond))
	}
	start = time.Now()

	if broken {
		return checkRecovered(files, errLists, softLists, r.lib, opts.Jobs, diags)
	}
	if opts.Metro && opts.MetroOptions["enabled"] != "false" && slices.ContainsFunc(files, check.UsesMetro) {
		r.lib.MetroOptions = opts.MetroOptions
		var err error
		if files, err = planMetro(files, srcs, frontend, r.lib, opts.Jobs); err != nil {
			return err
		}
		phase("metro")
	}
	checked, _ := check.CheckOptions(context.Background(), files, r.lib, opts.Jobs, nil, check.Options{Warnings: true,
		ExtraWarnings: opts.ExtraWarnings, ExperimentalWarnings: opts.ExperimentalWarnings})
	for _, e := range checked {
		if e.IsWarning() {
			*warnings = append(*warnings, e)
		} else {
			diags = append(diags, e)
		}
	}
	if len(diags) > 0 {
		return errors.Join(diags...)
	}
	phase("check")
	if opts.Timing != nil {
		<-prefetched
		if cp.HasKotlin() {
			fmt.Fprintf(opts.Timing, "%-8s %v (Kotlin default imports, after index, concurrent with parse and check)\n", "prefetch",
				prefetch.Round(time.Microsecond))
		}
		if r.shared {
			fmt.Fprintf(opts.Timing, "(load, kmeta and inline: over all the compiles that share the class path)\n")
		}
		n, d := cp.Loaded()
		fmt.Fprintf(opts.Timing, "%-8s %v (%d classes, part of check and prefetch)\n", "load", d.Round(time.Microsecond), n)
		if cp.HasKotlin() {
			n, d := cp.Kotlin.Get()
			fmt.Fprintf(opts.Timing, "%-8s %v (%d Kotlin declarations from metadata, part of check and prefetch)\n", "kmeta", d.Round(time.Microsecond), n)
			n, d = cp.Inline.Get()
			fmt.Fprintf(opts.Timing, "%-8s %v (%d bodies of inline functions read and analyzed, part of check)\n", "inline", d.Round(time.Microsecond), n)
		}
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}
	// The interfaces of the function types the program uses.
	var arities uint32
	for _, f := range files {
		arities |= f.Arities
	}
	// With the Kotlin standard library, function types are its interfaces.
	if arities != 0 && !cp.HasKotlin() {
		if err := os.MkdirAll(filepath.Join(opts.OutDir, filepath.FromSlash(types.FunctionPackage)), 0o755); err != nil {
			return err
		}
		for n := range types.MaxArity + 1 {
			if arities&(1<<n) != 0 {
				c := codegen.FunctionInterface(n)
				if err := os.WriteFile(filepath.Join(opts.OutDir, filepath.FromSlash(c.Name)+".class"), c.Bytes, 0o644); err != nil {
					return err
				}
			}
		}
	}
	genErrs := make([]error, len(files))
	hasFacade := make([]bool, len(files))
	// Classes go to directories by package, each created once.
	var dirs sync.Map
	mkdir := func(dir string) error {
		if _, ok := dirs.Load(dir); ok {
			return nil
		}
		err := os.MkdirAll(dir, 0o755)
		if err == nil {
			dirs.Store(dir, true)
		}
		return err
	}
	var writing atomic.Int64 // nanoseconds spent writing class files, over all workers
	var written atomic.Int64
	genOpts := codegen.Options{Stdlib: cp.HasKotlin(), Module: opts.ModuleName,
		Spilling: cp.Has("kotlin/coroutines/jvm/internal/SpillingKt") && !opts.Debug, Compose: opts.Compose,
		JVMDefaultCompat: opts.JVMDefaultCompat, Cache: &codegen.Cache{}}
	if opts.Compose {
		genOpts.ShouldExecute = hasMethod(cp, "androidx/compose/runtime/Composer", "shouldExecute", "(ZI)Z")
	}
	parts := make([]*cf.Class, len(files)) // the files' parts of multi-file classes
	parallel(len(files), opts.Jobs, func(i int) {
		classes, err := codegen.GenerateFile(files[i], genOpts)
		if err != nil {
			genErrs[i] = err
			return
		}
		for _, c := range classes {
			if c.Part != nil {
				parts[i] = c.Part
			}
		}
		start := time.Now()
		defer func() {
			writing.Add(int64(time.Since(start)))
			written.Add(int64(len(classes)))
		}()
		for _, c := range classes {
			hasFacade[i] = hasFacade[i] || c.Name == files[i].Class()
			path := filepath.Join(opts.OutDir, filepath.FromSlash(c.Name)+".class")
			if err := mkdir(filepath.Dir(path)); err != nil {
				genErrs[i] = err
				return
			}
			if err := os.WriteFile(path, c.Bytes, 0o644); err != nil {
				genErrs[i] = err
				return
			}
		}
	})
	if err := errors.Join(genErrs...); err != nil {
		return err
	}
	// The facades of multi-file classes, from all their parts.
	byFacade := map[string][]*cf.Class{}
	for i, f := range files {
		if parts[i] != nil {
			byFacade[f.Facade] = append(byFacade[f.Facade], parts[i])
		}
	}
	for name, ps := range byFacade {
		c, err := codegen.MultifileFacade(name, ps)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(opts.OutDir, filepath.FromSlash(c.Name)+".class"), c.Bytes, 0o644); err != nil {
			return err
		}
	}
	if err := writeModule(opts, files, hasFacade); err != nil {
		return err
	}
	phase("codegen")
	if opts.Timing != nil {
		fmt.Fprintf(opts.Timing, "%-8s %v (%d class files, part of codegen, summed over workers)\n", "write",
			time.Duration(writing.Load()).Round(time.Microsecond), written.Load())
		fmt.Fprintf(opts.Timing, "%-8s %v (Kotlin metadata, part of codegen, summed over workers)\n", "meta",
			time.Duration(codegen.MetaTime.Swap(0)).Round(time.Microsecond))
	}
	return nil
}

// writeModule writes META-INF/<module>.kotlin_module, which lists the
// file facades of each package, as kotlinc does: Kotlin compilers and
// reflection read it to find top-level declarations.
func writeModule(opts Options, files []*syntax.File, hasFacade []bool) error {
	facades := map[string][]kmeta.ModulePart{}
	for i, f := range files {
		if !hasFacade[i] {
			continue
		}
		pkg, name := "", f.Class()
		if j := strings.LastIndexByte(name, '/'); j >= 0 {
			pkg, name = strings.ReplaceAll(name[:j], "/", "."), name[j+1:]
		}
		part := kmeta.ModulePart{Name: name}
		if f.Part != "" {
			part.Facade = f.Facade[strings.LastIndexByte(f.Facade, '/')+1:]
		}
		facades[pkg] = append(facades[pkg], part)
	}
	module := opts.ModuleName
	if module == "" {
		module = "main"
	}
	dir := filepath.Join(opts.OutDir, "META-INF")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// As kotlinc names it: a Gradle module name, root:project, has a ':'.
	file := strings.ReplaceAll(module, ":", "_") + ".kotlin_module"
	return os.WriteFile(filepath.Join(dir, file), kmeta.WriteModule(facades), 0o644)
}

// hasMethod reports whether class name on class path cp has method
// name with descriptor desc.
func hasMethod(cp *classpath.Path, class, name, desc string) bool {
	info, err := cp.Load(class)
	if err != nil {
		return false
	}
	for _, m := range info.Methods {
		if m.Name == name && m.Desc == desc {
			return true
		}
	}
	return false
}

// addJava parses the module's Java sources and adds their classes to the
// class path, ahead of its own.
func addJava(cp *classpath.Path, paths []string, jobs int, outside *javaOutside) error {
	files := make([]*javasrc.File, len(paths))
	errs := make([]error, len(paths))
	parallel(len(paths), jobs, func(i int) {
		src, err := os.ReadFile(paths[i])
		if err != nil {
			errs[i] = err
			return
		}
		files[i], errs[i] = javasrc.Parse(paths[i], string(src))
		if files[i] != nil {
			// A member left out, as Java PSI leaves it, but for which
			// kotlinc goes on as well: javac reports Java's syntax
			// errors.
			errs[i] = nil
		}
	})
	if err := errors.Join(errs...); err != nil {
		return err
	}
	addJavaSet(cp, files, outside)
	return nil
}

// addJavaSet adds the classes of parsed Java sources files to the class
// path, ahead of its own, and returns them.
func addJavaSet(cp *classpath.Path, files []*javasrc.File, outside *javaOutside) *javasrc.Set {
	set := javasrc.NewSet(files)
	set.Outside = outside
	for _, name := range set.Names() {
		cp.AddSource(name, sync.OnceValues(func() (*cf.Info, error) { return set.Info(name) }))
	}
	return set
}

// JavaSources are a module's Java sources, whose classes AddJavaSources
// put over a class path.
type JavaSources struct {
	*javasrc.Set
	// Path is the class path with the sources' classes: an overlay of
	// the class path AddJavaSources was given, which it leaves as it is.
	Path    *classpath.Path
	outside *javaOutside
}

// AddJavaSources returns the classes of a module's parsed Java sources
// over class path cp, ahead of its own, which checks of other sources can
// go on sharing. The classes they name may be the module's Kotlin
// classes, which SetKotlin tells them of before any of their classes is
// read.
func AddJavaSources(cp *classpath.Path, files []*javasrc.File) *JavaSources {
	outside := &javaOutside{cp: cp}
	o := cp.Overlay()
	return &JavaSources{Set: addJavaSet(o, files, outside), Path: o, outside: outside}
}

// SetKotlin sets the module's Kotlin sources.
func (j *JavaSources) SetKotlin(files []*syntax.File) {
	j.outside.kotlin = kotlinClasses(files)
}

// javaOutside looks up the classes the Java sources name that aren't
// theirs: the class path's, and the program's Kotlin classes.
type javaOutside struct {
	cp     *classpath.Path
	kotlin map[string]*kotlinClass // by internal name
}

// kotlinClass is a class of the program's Kotlin sources, as the Java
// sources see it.
type kotlinClass struct {
	decl  *syntax.ClassDecl
	file  *syntax.File
	outer string // the internal name of the class it is nested in, or ""
}

func (o *javaOutside) Has(name string) bool {
	return o.kotlin[name] != nil || o.cp.Has(name) && !o.cp.IsSource(name)
}

func (o *javaOutside) Supers(name string) ([]string, bool) {
	if k := o.kotlin[name]; k != nil {
		// Its supertypes by the names written, which Java sources need
		// for the member types it inherits: KotlinInner3 in a Java
		// subclass of a Kotlin class whose superclass declares it. A
		// supertype with type arguments is left out: the inner classes
		// inherited through it would lose them, as SuperClass<String>'s
		// NestedInSuperClass, whose T the Java sources' signatures can't
		// give, so they stay unresolved.
		var supers []string
		for _, st := range k.decl.Supers {
			if st.Type == nil || st.Type.Name == "" || len(st.Type.Args) > 0 {
				continue
			}
			if n := o.kotlinType(k, st.Type.Name); n != "" {
				supers = append(supers, n)
			}
		}
		return supers, k.decl.Interface
	}
	info, err := o.cp.Load(name)
	if err != nil {
		return nil, false
	}
	supers := info.Interfaces
	if info.Super != "" {
		supers = append([]string{info.Super}, supers...)
	}
	return supers, info.Access&cf.AccInterface != 0
}

// exists reports whether a class of the program or the class path has an
// internal name.
func (o *javaOutside) exists(name string) bool {
	return o.kotlin[name] != nil || o.cp.Has(name)
}

// qualified returns the internal name of a class by its dotted name, with
// the longest package prefix it exists in, or "".
func (o *javaOutside) qualified(dotted string) string {
	segs := strings.Split(dotted, ".")
	for i := len(segs) - 1; i >= 0; i-- {
		n := strings.Join(segs[i:], "$")
		if i > 0 {
			n = strings.Join(segs[:i], "/") + "/" + n
		}
		if o.exists(n) {
			return n
		}
	}
	return ""
}

// kotlinType resolves the name of a type written in Kotlin class k's
// supertypes, without type arguments, to an internal name, or "". It
// looks in the classes k is nested in, the file's explicit imports, its
// package, its star imports and java.lang, and takes the name as
// qualified last. It doesn't know type aliases.
func (o *javaOutside) kotlinType(k *kotlinClass, written string) string {
	first, rest, _ := strings.Cut(written, ".")
	nested := func(n string) string {
		if n == "" || rest == "" {
			return n
		}
		n += "$" + strings.ReplaceAll(rest, ".", "$")
		if !o.exists(n) {
			return ""
		}
		return n
	}
	for outer := k.outer; outer != ""; outer = o.kotlin[outer].outer {
		if n := outer + "$" + first; o.exists(n) {
			return nested(n)
		}
	}
	f := k.file
	for _, imp := range f.Imports {
		if imp.Star {
			continue
		}
		name := imp.Alias
		if name == "" {
			name = imp.Path[strings.LastIndex(imp.Path, ".")+1:]
		}
		if name == first {
			return nested(o.qualified(imp.Path))
		}
	}
	pkg := ""
	if f.Package != "" {
		pkg = strings.ReplaceAll(f.Package, ".", "/") + "/"
	}
	if o.exists(pkg + first) {
		return nested(pkg + first)
	}
	for _, imp := range f.Imports {
		if !imp.Star {
			continue
		}
		if q := o.qualified(imp.Path + "." + first); q != "" {
			return nested(q)
		}
	}
	if o.exists("java/lang/" + first) {
		return nested("java/lang/" + first)
	}
	return o.qualified(written)
}

// IsKotlin reports whether a class is one of the program's Kotlin classes
// or facades, whose const vals the Java sources can't evaluate.
func (o *javaOutside) IsKotlin(class string) bool { return o.kotlin[class] != nil }

func (o *javaOutside) Constant(class, field string) (any, string) {
	if o.kotlin[class] != nil {
		return nil, ""
	}
	info, err := o.cp.Load(class)
	if err != nil {
		return nil, ""
	}
	for _, f := range info.Fields {
		if f.Name == field {
			return f.Constant, f.Desc
		}
	}
	return nil, ""
}

// kotlinClasses returns the program's classes by internal name, nested
// ones too, and its files' facades.
func kotlinClasses(files []*syntax.File) map[string]*kotlinClass {
	m := map[string]*kotlinClass{}
	var add func(f *syntax.File, outer, prefix string, d *syntax.ClassDecl)
	add = func(f *syntax.File, outer, prefix string, d *syntax.ClassDecl) {
		name := prefix + d.Name
		m[name] = &kotlinClass{decl: d, file: f, outer: outer}
		for _, n := range d.Members {
			if n, ok := n.(*syntax.ClassDecl); ok {
				add(f, name, name+"$", n)
			}
		}
	}
	for _, f := range files {
		prefix := ""
		if f.Package != "" {
			prefix = strings.ReplaceAll(f.Package, ".", "/") + "/"
		}
		for _, d := range f.Decls {
			if d, ok := d.(*syntax.ClassDecl); ok {
				add(f, "", prefix, d)
			}
		}
		if f.HasFacade() {
			m[f.Class()] = &kotlinClass{decl: &syntax.ClassDecl{}, file: f}
		}
	}
	return m
}

// frontendOf returns the parser opts pick.
func frontendOf(opts Options) (string, error) {
	f := opts.Frontend
	if f == "" {
		f = os.Getenv("KC_FRONTEND")
	}
	switch f {
	case "", "syntax":
		return "syntax", nil
	case "psi":
		return "psi", nil
	}
	return "", fmt.Errorf("unknown frontend %q: want syntax or psi", f)
}

// MetroPluginID is the id of Metro's compiler plugin, whose options
// MetroOptions are.
const MetroPluginID = "dev.zacsweers.metro.compiler"

// planMetro returns the files, parsed again from srcs, to check with what
// the Metro plugin adds to them, in lib, and the files of Kotlin it adds.
func planMetro(files []*syntax.File, srcs []string, frontend string, lib *check.Library, jobs int) ([]*syntax.File, error) {
	plan, errs := check.PlanMetro(files, lib)
	if len(errs) > 0 {
		diags := make([]error, len(errs))
		for i, e := range errs {
			diags[i] = e
		}
		return nil, errors.Join(diags...)
	}
	again := make([]*syntax.File, len(files), len(files)+len(plan.Files))
	parallel(len(files), jobs, func(i int) {
		again[i], _, _ = parse(frontend, files[i].Path, srcs[i])
	})
	for _, f := range plan.Files {
		pf, perrs := syntax.Parse(f.Path, f.Src)
		if len(perrs) > 0 {
			return nil, perrs[0]
		}
		again = append(again, pf)
	}
	lib.MetroPlan = plan
	return again, nil
}

// parse parses a source file with frontend. If it has syntax errors, it
// returns all of them, as kotlinc reports them (see syntaxErrors), with
// the file as far as parsing recovered: without the declarations and
// statements that failed, or nil if it can't. soft are the errors of kc's
// parser where it is stricter than kotlinc's, which checkRecovered
// reports unless the checker reports an error on the same line.
//
// kc's parser is the faster, and parses every file that compiles; a file
// it rejects is recovered from psi's tree, which has kotlinc's grammar.
func parse(frontend, path, src string) (f *syntax.File, errs, soft []*syntax.Error) {
	if frontend != "psi" {
		f, perrs := syntax.Parse(path, src)
		if len(perrs) == 0 {
			return f, nil, nil
		}
		s := psi.Parse(path, src)
		f, lerrs := lowerRecovering(s)
		errs, soft = syntaxErrors(path, src, s, perrs, lerrs)
		return f, errs, soft
	}
	s := psi.Parse(path, src)
	f, _, lerrs := lower.File(s)
	if len(lerrs) > 0 {
		// Checked as far as statement-level recovery gets, as with kc's
		// parser.
		rf, rerrs := lowerRecovering(s)
		errs, soft = syntaxErrors(path, src, s, lerrs, rerrs)
		return rf, errs, soft
	}
	return f, nil, nil
}

// lowerRecovering returns the file of psi tree s as lower.FileRecovering
// recovers it, and its errors, or nil if it can't.
func lowerRecovering(s *psi.Snapshot) (f *syntax.File, errs []*syntax.Error) {
	defer func() {
		// A file the token parser rejects can hold what lowering has no
		// case for; it is then left unchecked, with its syntax errors.
		if recover() != nil {
			f, errs = nil, nil
		}
	}()
	f, _, errs = lower.FileRecovering(s)
	return f, errs
}

// checkRecovered checks files, some of which have syntax errors, errLists
// by file, as far as the parser recovered them, and returns the syntax
// errors, diags, with the checker's errors on the lines without one. A
// declaration or statement with a syntax error is left out of its file,
// so the checker's errors on its lines would be about what parsing left.
// Its warnings are left out, as many are about what isn't used, which a
// declaration left out may use. The errors of softLists, kc's parser's
// where kotlinc's parser reports none, are reported only on lines the
// checker reports no error on: there, the checker's is kotlinc's error.
func checkRecovered(files []*syntax.File, errLists, softLists [][]*syntax.Error, lib *check.Library, jobs int, diags []error) error {
	syntaxLines := map[string]map[int]bool{}
	for i, es := range errLists {
		for _, e := range es {
			if syntaxLines[files[i].Path] == nil {
				syntaxLines[files[i].Path] = map[int]bool{}
			}
			syntaxLines[files[i].Path][e.Pos.Line] = true
		}
	}
	checked, _ := check.CheckOptions(context.Background(), files, lib, jobs, nil, check.Options{})
	checkerLines := map[string]map[int]bool{}
	for _, e := range checked {
		if !e.IsWarning() && !syntaxLines[e.File][e.Pos.Line] {
			diags = append(diags, e)
			if checkerLines[e.File] == nil {
				checkerLines[e.File] = map[int]bool{}
			}
			checkerLines[e.File][e.Pos.Line] = true
		}
	}
	for _, es := range softLists {
		for _, e := range es {
			if !checkerLines[e.File][e.Pos.Line] {
				diags = append(diags, e)
			}
		}
	}
	return errors.Join(diags...)
}
