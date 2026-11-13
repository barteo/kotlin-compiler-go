// Command kc compiles a subset of Kotlin to JVM class files.
//
//	kc [-d outdir] [-cp path] [-module-name name] [-parcelize] [-serialization] [-compose] [-metro] [-Xdebug] [-j N] [-time] [-features] files-or-dirs...
//	kc -serve
//
// kc -serve compiles again and again, as a tool that runs many small
// compiles (bench/conformance) asks: see serve.go.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-serve" || os.Args[1] == "--serve") {
		// The answers go to stdout, where what the compiler wrote would
		// garble them.
		out := os.Stdout
		os.Stdout = os.Stderr
		if err := serve(os.Stdin, out); err != nil {
			fmt.Fprintln(os.Stderr, "kc -serve:", err)
			os.Exit(1)
		}
		return
	}
	code, _ := compile(os.Args[1:], os.Stderr, nil)
	os.Exit(code)
}

// compile compiles as the command line args say, writing diagnostics to
// stderr, and returns the exit code and the compiler's error, if it ran.
// libs, if set, keeps class paths for later compiles: see
// driver.Libraries.
func compile(args []string, stderr io.Writer, libs *driver.Libraries) (int, error) {
	flags := flag.NewFlagSet("kc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outDir := flags.String("d", ".", "output directory for class files")
	jobs := flags.Int("j", runtime.NumCPU(), "number of parallel workers")
	timing := flags.Bool("time", false, "print per-phase timings to stderr")
	features := flags.Bool("features", false, "print diagnostics as path, line:col, unsupported feature and message, separated by tabs")
	var classPath string
	flags.StringVar(&classPath, "cp", "", "class path: jars and directories separated by "+string(os.PathListSeparator))
	flags.StringVar(&classPath, "classpath", "", "same as -cp")
	friendPaths := flags.String("friend-paths", "", "entries of the class path whose internal declarations are visible, as a unit test's main code (kotlinc's -Xfriend-paths), separated by "+string(os.PathListSeparator))
	jdkHome := flags.String("jdk-home", "", "JDK whose classes to use (default: $JAVA_HOME, or the java command on PATH)")
	noJDK := flags.Bool("no-jdk", false, "don't use JDK classes")
	moduleName := flags.String("module-name", "main", "the module's name, which internal members' JVM names end with")
	parcelize := flags.Bool("parcelize", false, "compile as kotlinc does with the parcelize compiler plugin: classes annotated @Parcelize get their Parcelable members")
	serialization := flags.Bool("serialization", false, "compile as kotlinc does with the kotlinx.serialization compiler plugin: classes annotated @Serializable get their serializers")
	compose := flags.Bool("compose", false, "compile as kotlinc does with the Compose compiler plugin: @Composable functions take a Composer and change flags")
	metro := flags.Bool("metro", false, "compile as kotlinc does with the Metro dependency injection compiler plugin: @DependencyGraph interfaces get implementations")
	debug := flags.Bool("Xdebug", false, "compile as kotlinc's -Xdebug: suspend functions keep dead variables' values for debuggers")
	var optIns []string
	flags.Func("opt-in", "opt in to the requirement marker with this qualified name, as kotlinc's -opt-in; may be repeated", func(v string) error {
		optIns = append(optIns, v)
		return nil
	})
	metroOptions := map[string]string{}
	flags.Func("P", "a compiler plugin's option, as kotlinc's -P: plugin:dev.zacsweers.metro.compiler:key=value, for -metro; may be repeated", func(v string) error {
		rest, ok := strings.CutPrefix(v, "plugin:"+driver.MetroPluginID+":")
		key, value, ok2 := strings.Cut(rest, "=")
		if !ok || !ok2 {
			return fmt.Errorf("unsupported plugin option %s: only plugin:%s:key=value", v, driver.MetroPluginID)
		}
		metroOptions[key] = value
		return nil
	})
	jvmDefault := flags.String("jvm-default", "no-compatibility", "how interfaces' bodies compile, as with kotlinc: no-compatibility (JVM default methods), enable (also in DefaultImpls classes) or disable (DefaultImpls classes)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: kc [-d outdir] [-cp path] [-module-name name] [-parcelize] [-serialization] [-compose] [-metro] [-j N] [-time] files-or-dirs...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2, nil
	}
	if flags.NArg() == 0 {
		flags.Usage()
		return 2, nil
	}

	var paths []string
	for _, arg := range flags.Args() {
		err := filepath.WalkDir(arg, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && (p == arg || strings.HasSuffix(p, ".kt")) {
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1, nil
		}
	}
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "kc: no .kt files in %s\n", strings.Join(flags.Args(), " "))
		return 1, nil
	}

	if *jvmDefault != "no-compatibility" && *jvmDefault != "enable" && *jvmDefault != "disable" {
		fmt.Fprintf(stderr, "kc: unsupported -jvm-default=%s: no-compatibility, enable or disable\n", *jvmDefault)
		return 2, nil
	}
	opts := driver.Options{OutDir: *outDir, Jobs: *jobs, JDKHome: *jdkHome, NoJDK: *noJDK, ModuleName: *moduleName,
		NoJVMDefault: *jvmDefault == "disable", JVMDefaultCompat: *jvmDefault == "enable", Parcelize: *parcelize, Serialization: *serialization, Compose: *compose, Metro: *metro, MetroOptions: metroOptions, Debug: *debug, OptIns: optIns,
		Libraries: libs}
	if classPath != "" {
		opts.ClassPath = filepath.SplitList(classPath)
	}
	if *friendPaths != "" {
		opts.FriendPaths = filepath.SplitList(*friendPaths)
	}
	if *timing {
		opts.Timing = stderr
	}
	if err := driver.Compile(paths, opts); err != nil {
		if *features {
			printFeatures(stderr, err)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 1, err
	}
	return 0, nil
}

// printFeatures prints each diagnostic of err as path, line:col, the
// unsupported feature it is about (empty for other errors) and the
// message, separated by tabs. Errors without a position are printed as
// they are.
func printFeatures(w io.Writer, err error) {
	var errs []error
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		errs = j.Unwrap()
	} else {
		errs = []error{err}
	}
	for _, e := range errs {
		var se *syntax.Error
		if errors.As(e, &se) {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", se.File, se.Pos, se.Feature, strings.ReplaceAll(se.Msg, "\n", " "))
		} else {
			fmt.Fprintln(w, e)
		}
	}
}
