// Command kc compiles a subset of Kotlin to JVM class files.
//
//	kc [-d outdir] [-cp path] [-module-name name] [-jvm-target version] [-parcelize] [-serialization] [-compose] [-metro] [-Xdebug] [-Werror] [-nowarn] [-Wextra] [-Xexplicit-api=mode] [-Xmulti-platform] [-j N] [-time] [-cpuprofile file] [-blockprofile file] [-features] files-or-dirs...
//	kc -serve
//	kc -version
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
	"runtime/debug"
	"runtime/pprof"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/driver"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// version is the release this kc was built as. The release workflow
// (.github/workflows/release.yml) sets it with
// -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		printVersion(os.Stdout)
		return
	}
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

// printVersion writes kc's version, the Go release it was built with and
// its platform. A build that isn't a release's names its commit, if Go
// recorded one.
func printVersion(w io.Writer) {
	v := version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			var rev, modified string
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					rev = s.Value
				case "vcs.modified":
					modified = s.Value
				}
			}
			if rev != "" {
				v += " (" + rev
				if modified == "true" {
					v += ", modified"
				}
				v += ")"
			}
		}
	}
	fmt.Fprintf(w, "kc %s %s %s/%s\n", v, runtime.Version(), runtime.GOOS, runtime.GOARCH)
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
	cpuProfile := flags.String("cpuprofile", "", "write a CPU profile of the compile to this file, for go tool pprof")
	blockProfile := flags.String("blockprofile", "", "write a profile of where goroutines waited during the compile to this file, for go tool pprof")
	features := flags.Bool("features", false, "print diagnostics as path, line:col, severity (error or warning), the diagnostic's name if known, unsupported feature and message, separated by tabs")
	werror := flags.Bool("Werror", false, "fail the compile if there are warnings, as kotlinc's -Werror")
	nowarn := flags.Bool("nowarn", false, "don't report warnings, as kotlinc's -nowarn")
	wextra := flags.Bool("Wextra", false, "also report the warnings of kotlinc's extra checkers, as kotlinc's -Wextra (UNREACHABLE_CODE)")
	experimental := flags.Bool("Xuse-fir-experimental-checkers", false, "also report the warnings of kotlinc's experimental checkers, as kotlinc's -Xuse-fir-experimental-checkers (REDUNDANT_INTERPOLATION_PREFIX)")
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
	// (Keep -Werror and -nowarn above: the flags from here are another
	// change's.)
	jvmTarget := flags.String("jvm-target", "1.8", "the JVM version the class files are for, as kotlinc's -jvm-target: 1.8, or 9 to 25. Inline functions of class files built for a higher one can't be inlined")
	explicitAPI := flags.String("Xexplicit-api", "disable", "kotlinc's explicit API mode: strict (errors) or warning (warnings) where a declaration of the public API doesn't say its visibility or its inferred type, or disable")
	explicitReturnTypes := flags.String("XXexplicit-return-types", "disable", "as kotlinc's -XXexplicit-return-types: strict (errors) or warning (warnings) where a declaration of the public API doesn't say its inferred type, or disable")
	multiPlatform := flags.Bool("Xmulti-platform", false, "compile multiplatform sources' expect and actual declarations, as kotlinc's -Xmulti-platform, without which they are errors")
	jvmDefault := flags.String("jvm-default", "no-compatibility", "how interfaces' bodies compile, as with kotlinc: no-compatibility (JVM default methods), enable (also in DefaultImpls classes) or disable (DefaultImpls classes)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: kc -version | kc [-d outdir] [-cp path] [-module-name name] [-parcelize] [-serialization] [-compose] [-metro] [-Werror] [-nowarn] [-Wextra] [-Xexplicit-api=mode] [-Xmulti-platform] [-j N] [-time] files-or-dirs...")
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
	apiMode, ok := check.ParseAPIMode(*explicitAPI)
	if !ok {
		fmt.Fprintf(stderr, "kc: unsupported -Xexplicit-api=%s: strict, warning or disable\n", *explicitAPI)
		return 2, nil
	}
	returnTypesMode, ok := check.ParseAPIMode(*explicitReturnTypes)
	if !ok {
		fmt.Fprintf(stderr, "kc: unsupported -XXexplicit-return-types=%s: strict, warning or disable\n", *explicitReturnTypes)
		return 2, nil
	}
	if _, ok := classfile.TargetMajor(*jvmTarget); !ok {
		fmt.Fprintf(stderr, "kc: unsupported -jvm-target %s: 1.8, or 9 to 25\n", *jvmTarget)
		return 2, nil
	}
	opts := driver.Options{OutDir: *outDir, JVMTarget: *jvmTarget, Jobs: *jobs, JDKHome: *jdkHome, NoJDK: *noJDK, ModuleName: *moduleName,
		NoJVMDefault: *jvmDefault == "disable", JVMDefaultCompat: *jvmDefault == "enable", Parcelize: *parcelize, Serialization: *serialization, Compose: *compose, Metro: *metro, MetroOptions: metroOptions, Debug: *debug, OptIns: optIns,
		ExtraWarnings: *wextra, ExperimentalWarnings: *experimental, ExplicitAPI: apiMode, ExplicitReturnTypes: returnTypesMode, MultiPlatform: *multiPlatform,
		Libraries: libs, JavaTypes: true}
	if classPath != "" {
		opts.ClassPath = filepath.SplitList(classPath)
	}
	if *friendPaths != "" {
		opts.FriendPaths = filepath.SplitList(*friendPaths)
	}
	if *timing {
		opts.Timing = stderr
	}
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1, nil
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(stderr, err)
			return 1, nil
		}
		defer pprof.StopCPUProfile()
	}
	if *blockProfile != "" {
		runtime.SetBlockProfileRate(1)
		defer func() {
			runtime.SetBlockProfileRate(0)
			if f, err := os.Create(*blockProfile); err == nil {
				pprof.Lookup("block").WriteTo(f, 0)
				f.Close()
			}
		}()
	}
	warnings, err := driver.CompileWarnings(paths, opts)
	if *nowarn {
		warnings = nil
	}
	for _, w := range warnings {
		if *features {
			printDiag(stderr, w)
		} else {
			fmt.Fprintln(stderr, w)
		}
	}
	if err != nil {
		if *features {
			printFeatures(stderr, err)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 1, err
	}
	if *werror && len(warnings) > 0 {
		fmt.Fprintln(stderr, "kc: warnings found and -Werror specified")
		return 1, nil
	}
	return 0, nil
}

// printFeatures prints each diagnostic of err as printDiag does. Errors
// without a position are printed as they are.
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
			printDiag(w, se)
		} else {
			fmt.Fprintln(w, e)
		}
	}
}

// printDiag prints diagnostic d as path, line:col, its severity (error
// or warning), its name if kc knows it, the unsupported feature it is
// about (empty for other diagnostics) and the message, separated by
// tabs, on a line.
func printDiag(w io.Writer, d *syntax.Error) {
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", d.File, d.Pos, d.Severity, d.Name, d.Feature, strings.ReplaceAll(d.Msg, "\n", " "))
}
