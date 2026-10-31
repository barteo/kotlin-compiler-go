// Package classpath finds and reads class files from the JDK's module image
// and from jars and directories on a class path.
//
// Opening a class path indexes the names of all classes, which is cheap.
// Class files are read and parsed only when the compiler asks for them.
package classpath

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// source reads the bytes of one indexed class file.
type source interface {
	read(e *entry) ([]byte, error)
	close() error
}

type entry struct {
	src  source
	off  int64 // jimage: offset of the resource
	size int64 // jimage: size of the resource
	file any   // jar: *zip.File; directory: path
	// friend is set on the classes of a friend path, a jar or directory
	// whose internal declarations the module sees, as a unit test sees its
	// main code's.
	friend bool
	// info returns the class of a source file, which has no class file:
	// see AddSource.
	info func() (*classfile.Info, error)
}

// Path is an indexed class path. It is safe for concurrent use once Open
// returns.
type Path struct {
	classes  map[string]entry // internal name -> where to read it
	packages map[string]bool  // internal package names, e.g. java/util
	// resources are the Kotlin metadata files, by path in the jar. A path
	// can appear in several jars: modules are often named alike (common,
	// or the empty module files of umbrella artifacts), so every copy is
	// kept, in class path order.
	resources map[string][]entry
	resNames  []string // keys of resources, in class path order
	sources   []source
	// sourceCount counts the classes of source files: see AddSource.
	sourceCount int

	// These caches belong to one compilation. The bytes and decoded objects
	// are immutable, and a concurrent first lookup performs the read once.
	bytes   sync.Map // string -> func() ([]byte, error)
	infos   sync.Map // string -> func() (*classfile.Info, error)
	methods sync.Map // string -> func() (*classfile.MethodReader, error)

	IndexTime time.Duration
	loaded    atomic.Int64
	loadNanos atomic.Int64

	// Kotlin counts the Kotlin declarations decoded from metadata and the
	// time spent decoding them, as reported by the checker.
	Kotlin Counter
	// Inline counts the bodies of inline functions read and analyzed, and
	// the time spent, as reported by the checker.
	Inline Counter
}

// Counter accumulates a count and a duration from several goroutines.
type Counter struct {
	n     atomic.Int64
	nanos atomic.Int64
}

// Add adds n items and the time d spent on them.
func (c *Counter) Add(n int, d time.Duration) {
	c.n.Add(int64(n))
	c.nanos.Add(int64(d))
}

// Get returns the totals.
func (c *Counter) Get() (int, time.Duration) {
	return int(c.n.Load()), time.Duration(c.nanos.Load())
}

// Open indexes the JDK in jdkHome (skipped if empty) followed by the given
// jars and directories. Earlier entries win when a class appears twice, as
// on the JVM. The entries also in friends are friend paths (kotlinc's
// -Xfriend-paths).
func Open(jdkHome string, entries []string, friends ...string) (*Path, error) {
	start := time.Now()
	// The JDK has about 28,000 classes in 1,000 packages.
	p := &Path{
		classes:   make(map[string]entry, 1<<15),
		packages:  make(map[string]bool, 1<<10),
		resources: make(map[string][]entry),
	}
	if jdkHome != "" {
		if err := p.addJImage(filepath.Join(jdkHome, "lib", "modules")); err != nil {
			p.Close()
			return nil, err
		}
	}
	for _, e := range entries {
		fi, err := os.Stat(e)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("class path: %w", err)
		}
		friend := false
		for _, f := range friends {
			friend = friend || filepath.Clean(f) == filepath.Clean(e)
		}
		if fi.IsDir() {
			err = p.addDir(e, friend)
		} else {
			err = p.addJar(e, friend)
		}
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("class path: %s: %w", e, err)
		}
	}
	p.IndexTime = time.Since(start)
	return p, nil
}

func (p *Path) add(name string, e entry) {
	if _, dup := p.classes[name]; dup {
		return
	}
	p.classes[name] = e
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		p.packages[name[:i]] = true
	} else {
		p.packages[""] = true
	}
}

// AddSource adds a class that a source file of the module declares, as a
// Java class of a module that mixes Java and Kotlin: info returns what
// its class file would hold. It hides a class of the same name on the
// class path, as kotlinc's sources do. It is called before the path is
// shared.
func (p *Path) AddSource(name string, info func() (*classfile.Info, error)) {
	p.classes[name] = entry{info: info}
	p.bytes.Delete(name)
	p.infos.Delete(name)
	p.methods.Delete(name)
	p.sourceCount++
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		p.packages[name[:i]] = true
	} else {
		p.packages[""] = true
	}
}

// HasSources reports whether AddSource added classes.
func (p *Path) HasSources() bool { return p.sourceCount > 0 }

// IsSource reports whether a class is one AddSource added.
func (p *Path) IsSource(name string) bool { return p.classes[name].info != nil }

// isResource reports whether a non-class file is Kotlin metadata the
// compiler reads: module files, which list each package's top-level
// declaration classes, and the descriptions of the built-in classes.
func isResource(name string) bool {
	return strings.HasSuffix(name, ".kotlin_builtins") ||
		strings.HasPrefix(name, "META-INF/") && strings.HasSuffix(name, ".kotlin_module") &&
			!strings.Contains(name[len("META-INF/"):], "/")
}

func (p *Path) addResource(name string, e entry) {
	if _, dup := p.resources[name]; !dup {
		p.resNames = append(p.resNames, name)
	}
	p.resources[name] = append(p.resources[name], e)
}

// Resource reads a Kotlin metadata file by its path in the jar or
// directory, e.g. kotlin/kotlin.kotlin_builtins. When several jars have
// it, the first on the class path wins, as for classes.
func (p *Path) Resource(name string) ([]byte, error) {
	es, ok := p.resources[name]
	if !ok {
		return nil, fmt.Errorf("%s not found", name)
	}
	return readResource(name, es[0])
}

// ResourceAll reads every copy of a Kotlin metadata file on the class
// path, in class path order.
func (p *Path) ResourceAll(name string) ([][]byte, error) {
	es, ok := p.resources[name]
	if !ok {
		return nil, fmt.Errorf("%s not found", name)
	}
	all := make([][]byte, len(es))
	for i, e := range es {
		data, err := readResource(name, e)
		if err != nil {
			return nil, err
		}
		all[i] = data
	}
	return all, nil
}

func readResource(name string, e entry) ([]byte, error) {
	data, err := e.src.read(&e)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return data, nil
}

// Resources returns the paths of the Kotlin metadata files whose names end
// in suffix, in class path order.
func (p *Path) Resources(suffix string) []string {
	var names []string
	for _, n := range p.resNames {
		if strings.HasSuffix(n, suffix) {
			names = append(names, n)
		}
	}
	return names
}

// HasKotlin reports whether the Kotlin standard library is on the class
// path: it describes the built-in classes such as kotlin.Int.
func (p *Path) HasKotlin() bool {
	_, ok := p.resources["kotlin/kotlin.kotlin_builtins"]
	return ok
}

// Close releases open files.
func (p *Path) Close() error {
	var errs []error
	for _, s := range p.sources {
		errs = append(errs, s.close())
	}
	return errors.Join(errs...)
}

// Len returns the number of indexed classes.
func (p *Path) Len() int { return len(p.classes) }

// Has reports whether a class exists, by internal name (java/util/Random).
func (p *Path) Has(name string) bool {
	_, ok := p.classes[name]
	return ok
}

// HasPackage reports whether any class is in the package, by internal name
// (java/util).
func (p *Path) HasPackage(pkg string) bool { return p.packages[pkg] }

// Bytes reads a class file by internal name. The returned bytes are shared
// for the lifetime of Path and must not be modified.
func (p *Path) Bytes(name string) ([]byte, error) {
	e, ok := p.classes[name]
	if !ok {
		return nil, fmt.Errorf("class %s not found", name)
	}
	if e.info != nil {
		return nil, fmt.Errorf("class %s is in a source file", name)
	}
	if load, ok := p.bytes.Load(name); ok {
		return load.(func() ([]byte, error))()
	}
	load := sync.OnceValues(func() ([]byte, error) {
		data, err := e.src.read(&e)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		return data, nil
	})
	actual, _ := p.bytes.LoadOrStore(name, load)
	return actual.(func() ([]byte, error))()
}

// MethodReader indexes a class's methods once and decodes their bodies on
// demand. It shares the class bytes and constant pool across inline calls.
func (p *Path) MethodReader(name string) (*classfile.MethodReader, error) {
	if load, ok := p.methods.Load(name); ok {
		return load.(func() (*classfile.MethodReader, error))()
	}
	load := sync.OnceValues(func() (*classfile.MethodReader, error) {
		data, err := p.Bytes(name)
		if err != nil {
			return nil, err
		}
		return classfile.NewMethodReader(data)
	})
	actual, _ := p.methods.LoadOrStore(name, load)
	return actual.(func() (*classfile.MethodReader, error))()
}

// Friend reports whether a class is in a friend path, by internal name.
func (p *Path) Friend(name string) bool { return p.classes[name].friend }

// Load reads and parses a class by internal name. The result is shared and
// must not be modified. Source declarations are supplied by their owner.
func (p *Path) Load(name string) (*classfile.Info, error) {
	if e := p.classes[name]; e.info != nil {
		return e.info()
	}
	if load, ok := p.infos.Load(name); ok {
		return load.(func() (*classfile.Info, error))()
	}
	load := sync.OnceValues(func() (*classfile.Info, error) {
		start := time.Now()
		data, err := p.Bytes(name)
		if err != nil {
			return nil, err
		}
		info, err := classfile.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		p.loaded.Add(1)
		p.loadNanos.Add(int64(time.Since(start)))
		return info, nil
	})
	actual, _ := p.infos.LoadOrStore(name, load)
	return actual.(func() (*classfile.Info, error))()
}

// Loaded returns how many classes Load has read and the total time spent,
// summed over goroutines.
func (p *Path) Loaded() (int, time.Duration) {
	return int(p.loaded.Load()), time.Duration(p.loadNanos.Load())
}

// FindJDK returns the home directory of a JDK with a module image: JAVA_HOME
// if set, otherwise the JDK of the java command on PATH.
func FindJDK() (string, error) {
	isJDK := func(home string) bool {
		_, err := os.Stat(filepath.Join(home, "lib", "modules"))
		return err == nil
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		if isJDK(home) {
			return home, nil
		}
		return "", fmt.Errorf("JAVA_HOME=%s has no lib/modules", home)
	}
	java, err := exec.LookPath("java")
	if err == nil {
		if java, err = filepath.EvalSymlinks(java); err == nil {
			if home := filepath.Dir(filepath.Dir(java)); isJDK(home) {
				return home, nil
			}
		}
	}
	return "", errors.New("no JDK found: set JAVA_HOME or use -jdk-home")
}

// FindStdlib returns the path of kotlin-stdlib.jar: in KOTLIN_HOME/lib if
// set, otherwise next to the kotlinc command on PATH.
func FindStdlib() (string, error) {
	jar := func(home string) string {
		p := filepath.Join(home, "lib", "kotlin-stdlib.jar")
		if _, err := os.Stat(p); err != nil {
			return ""
		}
		return p
	}
	if home := os.Getenv("KOTLIN_HOME"); home != "" {
		if p := jar(home); p != "" {
			return p, nil
		}
		return "", fmt.Errorf("KOTLIN_HOME=%s has no lib/kotlin-stdlib.jar", home)
	}
	kotlinc, err := exec.LookPath("kotlinc")
	if err == nil {
		if kotlinc, err = filepath.EvalSymlinks(kotlinc); err == nil {
			// bin/kotlinc, in a distribution or in Homebrew's libexec.
			home := filepath.Dir(filepath.Dir(kotlinc))
			for _, h := range []string{home, filepath.Join(home, "libexec")} {
				if p := jar(h); p != "" {
					return p, nil
				}
			}
		}
	}
	return "", errors.New("kotlin-stdlib.jar not found: set KOTLIN_HOME")
}
