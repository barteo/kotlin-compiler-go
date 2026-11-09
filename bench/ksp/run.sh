#!/usr/bin/env bash
# run.sh times KSP on a project with stock KSP 2.3.6 and with the KSP fork
# on kc (jvm/kc-aa), each in a JVM of its own, run after run (KspBench.kt),
# and then each cold, in fresh JVMs.
#
#	bench/ksp/run.sh SRC_DIR[:SRC_DIR...] LIBS_FILE PROCS_FILE [RUNS]
#
# LIBS_FILE and PROCS_FILE list the module's libraries and the processors'
# class path, one jar per line (Gradle's, as the profiling session wrote
# them). RUNS defaults to 20. It needs jvm/kc-aa/build.sh stock to have run
# (for kc-aa.jar, libkc.so and the stock KSP jars). Environment:
#	BACKENDS  the backends to time (default "stock kc")
#	COLD      fresh-JVM runs per backend (default 3)
#	JFR       record bench-BACKEND.jfr from this run on (see Classify.java)
#	KC_TIMING set to 1 for kc's own phase split, on stderr
#	DUMP      first write what the processors see to DUMP-BACKEND.txt
#	          (KspBench's bench.dump), for comparing the backends; with
#	          RUNS_AFTER_DUMP=0 nothing is timed; with DUMP_RUNS=N the
#	          dump is the last of N runs in one JVM (a warm daemon's)
#	EDIT      edit this source (relative to the first source directory,
#	          or "auto") before each run after the first, with incremental
#	          processing on (KspBench's bench.edit)
#	EDIT_KIND body or signature for those edits only (KspBench's bench.editKind)
#	JVM_OPTS  more options for the benchmark's JVMs; @B@ stands for the
#	          backend, as in -XX:SharedArchiveFile=cds-@B@.jsa
#	STOCK_PROCS_EXTRA  a class path stock KSP's processors also need,
#	          which the KSP fork brings itself (Room's kotlinx-serialization)
set -euo pipefail

[ $# -ge 3 ] || { sed -n '2,15p' "$0" >&2; exit 2; }
srcs=$1 libsfile=$(readlink -f "$2") procsfile=$(readlink -f "$3") runs=${4:-20}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
kcaa=$root/jvm/kc-aa
out=$kcaa/build
export JAVA_TOOL_OPTIONS=
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}

kspwork=$root/jvm/ksp/build
lib=$kspwork/aa/m2
m2=$kspwork/m2
runtime=$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar
deps=$kspwork/aa/kc-aa-api-2.4.21.jar:$runtime:$lib/caffeine-2.9.3.jar:$kspwork/ksp-aa-kc.jar
deps=$deps:$m2/symbol-processing-api-2.3.9.jar:$m2/symbol-processing-common-deps-2.3.9.jar
deps=$deps:$m2/kotlinx-serialization-core-jvm-1.6.3.jar:$m2/kotlinx-serialization-json-jvm-1.6.3.jar
stockcp=
for jar in symbol-processing-aa-embeddable-2.3.6 symbol-processing-api-2.3.6 symbol-processing-common-deps-2.3.6 \
	kotlin-stdlib-2.3.0 kotlinx-coroutines-core-jvm-1.8.0-intellij-14; do
	[ -s "$out/m2/$jar.jar" ] || { echo "run.sh: run jvm/kc-aa/build.sh stock first" >&2; exit 1; }
	stockcp=$stockcp:$out/m2/$jar.jar
done
[ -s "$out/kc-aa.jar" ] && [ -s "$out/libkc.so" ] || { echo "run.sh: run jvm/kc-aa/build.sh first" >&2; exit 1; }
kotlinc() { java -Xmx3g -cp "$runtime" org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -no-stdlib -no-reflect "$@"; }

# The processors, but for what KSP itself brings.
procs=$({ grep -v 'symbol-processing-api\|kotlin-stdlib\|kotlin-reflect' "$procsfile" || true; } | paste -sd:)

for b in ${BACKENDS:-stock kc}; do
	jvmopts=${JVM_OPTS:-}
	jvmopts=${jvmopts//@B@/$b}
	[ -n "${EDIT:-}" ] && jvmopts+=" -Dbench.edit=$EDIT"
	[ -n "${EDIT_KIND:-}" ] && jvmopts+=" -Dbench.editKind=$EDIT_KIND"
	case $b in
	stock)
		[ -s "$out/bench-stock.jar" ] && [ "$out/bench-stock.jar" -nt "$here/KspBench.kt" ] ||
			kotlinc -cp "${stockcp#:}" -language-version 2.3 -api-version 2.3 -d "$out/bench-stock.jar" -jvm-target 17 -nowarn "$here/KspBench.kt"
		cmd=(java $jvmopts -Dksp.languageVersion=2.3 -cp "$out/bench-stock.jar$stockcp:$procs${STOCK_PROCS_EXTRA:+:$STOCK_PROCS_EXTRA}")
		;;
	kc)
		[ -s "$out/bench-kc.jar" ] && [ "$out/bench-kc.jar" -nt "$here/KspBench.kt" ] ||
			kotlinc -cp "$deps" -d "$out/bench-kc.jar" -jvm-target 17 -nowarn "$here/KspBench.kt"
		# The processors come first: kotlin-compiler bundles an older Guava.
		cmd=(env LD_PRELOAD="$java_home/lib/libjsig.so" java $jvmopts -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out"
			-cp "$out/bench-kc.jar:$procs:$out/kc-aa.jar:$deps")
		;;
	*) echo "run.sh: unknown backend $b" >&2; exit 2 ;;
	esac
	if [ -n "${DUMP:-}" ]; then
		"${cmd[@]}" -Dbench.dump="$DUMP-$b.txt" KspBenchKt "$srcs" "$libsfile" "${DUMP_RUNS:-1}" | grep -v '^summary'
		[ "${RUNS_AFTER_DUMP:-1}" = 0 ] && continue
	fi
	jfr=()
	[ -n "${JFR:-}" ] && jfr=(-Djfr.from="$JFR")
	"${cmd[@]}" "${jfr[@]}" KspBenchKt "$srcs" "$libsfile" "$runs"
	[ -n "${JFR:-}" ] && mv bench.jfr "bench-$b.jfr"
	for i in $(seq "${COLD:-3}"); do
		"${cmd[@]}" KspBenchKt "$srcs" "$libsfile" 1 | grep '^summary' | sed 's/^summary/cold/'
	done
done
