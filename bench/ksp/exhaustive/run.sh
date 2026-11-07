#!/usr/bin/env bash
# run.sh runs KSP's TachiyomiExhaustive benchmark with bench/ksp/run.sh:
# upstream's ExhaustiveProcessor over Tachiyomi's 559 Kotlin files and 136
# jars, on stock KSP2 and on the KSP fork on kc.
#
#	bench/ksp/exhaustive/run.sh [RUNS]
#
# It runs fetch.sh first, and builds the processor into
# jvm/kc-aa/build/exhaustive/processor.jar, compiled against stock KSP's
# API 2.3.6 and Kotlin 2.3 so that both backends load the same jar. The
# environment is bench/ksp/run.sh's (BACKENDS, COLD, JFR, KC_TIMING,
# JVM_OPTS).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
"$here/fetch.sh"
data=$here/data
export JAVA_TOOL_OPTIONS=
lib=$root/jvm/ksp/build/aa/m2
m2=$root/jvm/kc-aa/build/m2
work=$root/jvm/kc-aa/build/exhaustive
jar=$work/processor.jar
[ -s "$m2/symbol-processing-api-2.3.6.jar" ] || { echo "run.sh: run jvm/kc-aa/build.sh stock first" >&2; exit 1; }
if [ ! -s "$jar" ] || [ "$here/ExhaustiveProcessor.kt" -nt "$jar" ]; then
	rm -rf "$work/classes"
	mkdir -p "$work/classes"
	java -Xmx3g -cp "$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar" \
		org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -no-stdlib -no-reflect \
		-cp "$m2/symbol-processing-api-2.3.6.jar:$m2/kotlin-stdlib-2.3.0.jar" \
		-language-version 2.3 -api-version 2.3 -jvm-target 17 -nowarn \
		-d "$work/classes" "$here/ExhaustiveProcessor.kt"
	cp -r "$here/META-INF" "$work/classes/"
	(cd "$work/classes" && jar cf "$jar" .)
fi
echo "$jar" >"$work/procs.txt"
exec "$root/bench/ksp/run.sh" "$data/src" "$data/libs.txt" "$work/procs.txt" "$@"
