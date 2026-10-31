// Package driver runs the compilation pipeline.
package driver

import (
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
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

type Options struct {
	OutDir    string
	Jobs      int
	Timing    io.Writer // if set, per-phase timings are written here
	ClassPath []string  // jars and directories, searched after the JDK
	// FriendPaths are the entries of ClassPath whose internal declarations
	// the module sees, as with kotlinc's -Xfriend-paths: a unit test's main
	// code.
	FriendPaths []string
	JDKHome     string // JDK whose classes are used; found automatically if empty
	NoJDK       bool   // don't use JDK classes
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
	// Debug compiles as kotlinc's -Xdebug: suspend functions save dead
	// variables' values in their continuations as they are, not nulled
	// out, for debuggers.
	Debug bool
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

// Compile compiles the given .kt files into class files in opts.OutDir.
// The .java files among them are the module's Java sources, whose
// declarations the Kotlin code sees, as kotlinc's; javac compiles them.
// Diagnostics are returned as a joined error.
func Compile(paths []string, opts Options) error {
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
		cp  *classpath.Path
		lib *check.Library
		err error
	}
	cpDone := make(chan cpResult, 1)
	// The classes the Java sources name that aren't theirs. The program's
	// Kotlin classes are added once they are parsed, before checking reads
	// any Java source's class.
	java := &javaOutside{}
	var prefetch time.Duration
	prefetched := make(chan struct{})
	go func() {
		jdk := opts.JDKHome
		if jdk == "" && !opts.NoJDK {
			// Without a JDK, programs can use everything but Java classes.
			jdk, _ = classpath.FindJDK()
		}
		cp, err := classpath.Open(jdk, opts.ClassPath, opts.FriendPaths...)
		if err == nil && len(javaPaths) > 0 {
			err = addJava(cp, javaPaths, opts.Jobs, java)
		}
		var lib *check.Library
		if err == nil {
			lib = check.NewLibrary(cp)
			lib.Module = opts.ModuleName
			lib.NoJVMDefault = opts.NoJVMDefault
			lib.JVMDefaultCompat = opts.JVMDefaultCompat
			lib.Parcelize = opts.Parcelize
			lib.Compose = opts.Compose
			if opts.Serialization {
				lib.EnableSerialization()
			}
		}
		cpDone <- cpResult{cp, lib, err}
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
			r.cp.Close()
		}
	}()

	files := make([]*syntax.File, len(paths))
	errLists := make([][]*syntax.Error, len(paths))
	ioErrs := make([]error, len(paths))
	parallel(len(paths), opts.Jobs, func(i int) {
		src, err := os.ReadFile(paths[i])
		if err != nil {
			ioErrs[i] = err
			return
		}
		files[i], errLists[i] = syntax.Parse(paths[i], string(src))
	})
	if err := errors.Join(ioErrs...); err != nil {
		return err
	}
	var diags []error
	for _, es := range errLists {
		for _, e := range es {
			diags = append(diags, e)
		}
	}
	if len(diags) > 0 {
		return errors.Join(diags...)
	}
	classes := map[string]string{} // the files' facades and parts, by name
	multifile := map[string]bool{} // the multi-file classes' facades
	for _, f := range files {
		if !f.HasFacade() {
			continue // a file of expect declarations, as a common one
		}
		if prev, dup := classes[f.Class()]; dup {
			return fmt.Errorf("%s: duplicate JVM class name %s (also from %s)", f.Path, f.Class(), prev)
		}
		classes[f.Class()] = f.Path
		if f.Part != "" {
			multifile[f.Facade] = true
		}
	}
	for name := range multifile {
		if prev, dup := classes[name]; dup {
			return fmt.Errorf("%s: duplicate JVM class name %s (also a multi-file class)", prev, name)
		}
	}
	phase("parse")
	r := openCP()
	if r.err != nil {
		return r.err
	}
	cp := r.cp
	java.cp = cp
	if len(javaPaths) > 0 {
		java.kotlin = kotlinClasses(files)
	}
	if opts.Timing != nil {
		fmt.Fprintf(opts.Timing, "%-8s %v (%d classes, concurrent with parse; waited %v after it)\n", "index",
			cp.IndexTime.Round(time.Microsecond), cp.Len(), time.Since(start).Round(time.Microsecond))
	}
	start = time.Now()

	for _, e := range check.CheckWith(files, r.lib, opts.Jobs) {
		diags = append(diags, e)
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
		JVMDefaultCompat: opts.JVMDefaultCompat}
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
	})
	if err := errors.Join(errs...); err != nil {
		return err
	}
	set := javasrc.NewSet(files)
	set.Outside = outside
	for _, name := range set.Names() {
		cp.AddSource(name, func() (*cf.Info, error) { return set.Info(name) })
	}
	return nil
}

// javaOutside looks up the classes the Java sources name that aren't
// theirs: the class path's, and the program's Kotlin classes.
type javaOutside struct {
	cp     *classpath.Path
	kotlin map[string]*syntax.ClassDecl // by internal name
}

func (o *javaOutside) Has(name string) bool {
	return o.kotlin[name] != nil || o.cp.Has(name) && !o.cp.IsSource(name)
}

func (o *javaOutside) Supers(name string) ([]string, bool) {
	if d := o.kotlin[name]; d != nil {
		return nil, d.Interface
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
func kotlinClasses(files []*syntax.File) map[string]*syntax.ClassDecl {
	m := map[string]*syntax.ClassDecl{}
	var add func(prefix string, d *syntax.ClassDecl)
	add = func(prefix string, d *syntax.ClassDecl) {
		name := prefix + d.Name
		m[name] = d
		for _, n := range d.Members {
			if n, ok := n.(*syntax.ClassDecl); ok {
				add(name+"$", n)
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
				add(prefix, d)
			}
		}
		if f.HasFacade() {
			m[f.Class()] = &syntax.ClassDecl{}
		}
	}
	return m
}
