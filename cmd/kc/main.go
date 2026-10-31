// Command kc compiles a subset of Kotlin to JVM class files.
//
//	kc [-d outdir] [-cp path] [-module-name name] [-parcelize] [-serialization] [-compose] [-Xdebug] [-j N] [-time] [-features] files-or-dirs...
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

func main() {
	outDir := flag.String("d", ".", "output directory for class files")
	jobs := flag.Int("j", runtime.NumCPU(), "number of parallel workers")
	timing := flag.Bool("time", false, "print per-phase timings to stderr")
	features := flag.Bool("features", false, "print diagnostics as path, line:col, unsupported feature and message, separated by tabs")
	var classPath string
	flag.StringVar(&classPath, "cp", "", "class path: jars and directories separated by "+string(os.PathListSeparator))
	flag.StringVar(&classPath, "classpath", "", "same as -cp")
	friendPaths := flag.String("friend-paths", "", "entries of the class path whose internal declarations are visible, as a unit test's main code (kotlinc's -Xfriend-paths), separated by "+string(os.PathListSeparator))
	jdkHome := flag.String("jdk-home", "", "JDK whose classes to use (default: $JAVA_HOME, or the java command on PATH)")
	noJDK := flag.Bool("no-jdk", false, "don't use JDK classes")
	moduleName := flag.String("module-name", "main", "the module's name, which internal members' JVM names end with")
	parcelize := flag.Bool("parcelize", false, "compile as kotlinc does with the parcelize compiler plugin: classes annotated @Parcelize get their Parcelable members")
	serialization := flag.Bool("serialization", false, "compile as kotlinc does with the kotlinx.serialization compiler plugin: classes annotated @Serializable get their serializers")
	compose := flag.Bool("compose", false, "compile as kotlinc does with the Compose compiler plugin: @Composable functions take a Composer and change flags")
	debug := flag.Bool("Xdebug", false, "compile as kotlinc's -Xdebug: suspend functions keep dead variables' values for debuggers")
	jvmDefault := flag.String("jvm-default", "no-compatibility", "how interfaces' bodies compile, as with kotlinc: no-compatibility (JVM default methods), enable (also in DefaultImpls classes) or disable (DefaultImpls classes)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: kc [-d outdir] [-cp path] [-module-name name] [-parcelize] [-serialization] [-compose] [-j N] [-time] files-or-dirs...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var paths []string
	for _, arg := range flag.Args() {
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
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if *jvmDefault != "no-compatibility" && *jvmDefault != "enable" && *jvmDefault != "disable" {
		fmt.Fprintf(os.Stderr, "kc: unsupported -jvm-default=%s: no-compatibility, enable or disable\n", *jvmDefault)
		os.Exit(2)
	}
	opts := driver.Options{OutDir: *outDir, Jobs: *jobs, JDKHome: *jdkHome, NoJDK: *noJDK, ModuleName: *moduleName,
		NoJVMDefault: *jvmDefault == "disable", JVMDefaultCompat: *jvmDefault == "enable", Parcelize: *parcelize, Serialization: *serialization, Compose: *compose, Debug: *debug}
	if classPath != "" {
		opts.ClassPath = filepath.SplitList(classPath)
	}
	if *friendPaths != "" {
		opts.FriendPaths = filepath.SplitList(*friendPaths)
	}
	if *timing {
		opts.Timing = os.Stderr
	}
	if err := driver.Compile(paths, opts); err != nil {
		if *features {
			printFeatures(err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

// printFeatures prints each diagnostic of err as path, line:col, the
// unsupported feature it is about (empty for other errors) and the
// message, separated by tabs. Errors without a position are printed as
// they are.
func printFeatures(err error) {
	var errs []error
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		errs = j.Unwrap()
	} else {
		errs = []error{err}
	}
	for _, e := range errs {
		var se *syntax.Error
		if errors.As(e, &se) {
			fmt.Fprintf(os.Stderr, "%s\t%s\t%s\t%s\n", se.File, se.Pos, se.Feature, strings.ReplaceAll(se.Msg, "\n", " "))
		} else {
			fmt.Fprintln(os.Stderr, e)
		}
	}
}
