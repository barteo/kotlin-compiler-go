#!/bin/sh
# Times compiling kotlinx-coroutines-core (common, concurrent and jvm)
# with kc and kotlinc, and writes bench/results/time-coroutines.md.
#
#   bench/coroutines-time.sh                       # about 2 minutes
#   RUNS=5 WARM_RUNS=20 bench/coroutines-time.sh
#
# Both compilers get the build's options: the module name and
# -jvm-default=enable, and kotlinc also the others (explicit API mode,
# opt-ins, -progressive, no assertions, -jvm-target 1.8); kotlinc compiles
# the three source sets as fragments of a multiplatform module, as Gradle
# does. RUNS: cold kotlinc runs (kc always gets 10). WARM_RUNS: kotlinc
# runs in one JVM, of which the median of the second half is reported.
#
# Needs the same tools as bench/run.sh, the clone that bench/survey.sh
# makes, and for the first run network access to Maven Central, for the
# library's dependencies.
# COOLDOWN: seconds to idle before each run, outside the measured time (default 0).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
tag=${TAG:-1.11.0}
src="$TMPDIR/kc-survey/kotlinx-coroutines-$tag/kotlinx-coroutines-core"
deps="$TMPDIR/kc-coroutines-deps"
work="$TMPDIR/kc-coroutines-time"
results="$root/bench/results"
runs=${RUNS:-3}
cooldown=${COOLDOWN:-0}
warm_runs=${WARM_RUNS:-10}
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
stdlib="$kotlin_home/lib/kotlin-stdlib.jar"
cpus=$(getconf _NPROCESSORS_ONLN)
export JAVA_OPTS="-Xmx${HEAP:-8g}"
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
export PATH="$JAVA_HOME/bin:$PATH"

[ -d "$src" ] || { echo "no kotlinx.coroutines clone at $src: run bench/survey.sh first"; exit 1; }
mkdir -p "$deps"
cp=$stdlib
for jar in org/jetbrains/kotlinx/atomicfu-jvm/0.32.1/atomicfu-jvm-0.32.1.jar \
	com/google/android/annotations/4.1.1.4/annotations-4.1.1.4.jar \
	org/codehaus/mojo/animal-sniffer-annotations/1.24/animal-sniffer-annotations-1.24.jar; do
	f="$deps/$(basename "$jar")"
	[ -f "$f" ] || curl -fsSL -o "$f" "https://repo1.maven.org/maven2/$jar"
	cp="$cp:$f"
done
rm -rf "$work"
mkdir -p "$work/warm" "$results"
(cd "$root" && go build -o "$work/kc" ./cmd/kc)
javac -d "$work/warm" "$root/bench/warm/WarmKotlinc.java"

sets="common concurrent jvm"
dirs=$(for s in $sets; do echo "$src/$s/src"; done)
all=$(find $dirs -name '*.kt' | sort)
files=$(echo "$all" | wc -l | tr -d ' ')
lines=$(cat $all | wc -l | tr -d ' ')
fragments=$(for s in $sets; do find "$src/$s/src" -name '*.kt' | sort | sed "s|^|$s:|"; done | paste -sd, -)
module=kotlinx-coroutines-core
opts="-module-name $module -jvm-default=enable"
optins=kotlin.ExperimentalMultiplatform,kotlin.experimental.ExperimentalTypeInference,kotlinx.coroutines.DelicateCoroutinesApi,kotlinx.coroutines.ExperimentalCoroutinesApi,kotlinx.coroutines.ObsoleteCoroutinesApi,kotlinx.coroutines.InternalCoroutinesApi,kotlinx.coroutines.FlowPreview
kotlinc_args="-nowarn -Xmulti-platform -Xexpect-actual-classes -Xfragments=common,concurrent,jvm
	-Xfragment-refines=concurrent:common,jvm:concurrent -Xfragment-sources=$fragments
	-progressive -Xexplicit-api=strict -Xdont-warn-on-error-suppression -Xno-param-assertions -Xno-call-assertions
	-Xno-receiver-assertions -jvm-target 1.8 -opt-in=$optins -cp $cp $opts"

# shellcheck disable=SC2086 # the arguments are words
hyperfine --warmup 1 --runs 10 --style basic --export-json "$work/kc.json" \
	--prepare "rm -rf $work/out-kc && sleep $cooldown" \
	-n "kc -j 1" "$work/kc -j 1 $opts -cp $cp -d $work/out-kc $(echo $dirs)" \
	-n "kc -j $cpus" "$work/kc -j $cpus $opts -cp $cp -d $work/out-kc $(echo $dirs)"
# shellcheck disable=SC2086
hyperfine --runs "$runs" --style basic --export-json "$work/kotlinc.json" \
	--prepare "rm -rf $work/out-kotlinc && sleep $cooldown" \
	-n "kotlinc (cold JVM)" "kotlinc $(echo $kotlinc_args) -d $work/out-kotlinc $(echo $all)"
python3 -c 'import json, sys
r = [x for f in sys.argv[2:] for x in json.load(open(f))["results"]]
json.dump({"results": r}, open(sys.argv[1], "w"), indent=2)' \
	"$results/hyperfine-coroutines.json" "$work/kc.json" "$work/kotlinc.json"

# shellcheck disable=SC2086
warm=$(java $JAVA_OPTS -Dcooldown="$cooldown" -cp "$work/warm:$kotlin_home/lib/*" WarmKotlinc "$warm_runs" \
	$kotlinc_args -kotlin-home "$kotlin_home" -d "$work/out-warm" $all 2>/dev/null |
	tee "$results/warm-coroutines.txt" | tail -1)
echo "kotlinc (warm JVM): $warm"
classes_kc=$(find "$work/out-kc" -name '*.class' | wc -l | tr -d ' ')
classes_kotlinc=$(find "$work/out-kotlinc" -name '*.class' | wc -l | tr -d ' ')

{
	echo "# kc vs kotlinc on kotlinx-coroutines-core"
	echo
	echo "Kotlin/kotlinx.coroutines $tag's kotlinx-coroutines-core for the JVM: $files files, $lines lines (common, concurrent and jvm)."
	echo
	echo "- Machine: $(uname -sm), $cpus CPUs"
	echo "- $(go version)"
	echo "- $(java -version 2>&1 | grep -v JAVA_TOOL | head -1)"
	echo "- $(kotlinc -version 2>&1 | grep -v JAVA_TOOL | tail -1)"
	echo "- kotlinc JVM options: $JAVA_OPTS"
	echo "- Cooldown before each run (outside timing): $cooldown s"
	echo
	python3 "$root/bench/summarize.py" "$results/hyperfine-coroutines.json" "kc -j 1" "kc -j $cpus" "kotlinc (cold JVM)"
	echo
	echo "- kotlinc in a warm JVM (median of the last $((warm_runs - warm_runs / 2)) of $warm_runs runs in one JVM): ${warm#warm median: }"
	echo "- Class files: kc $classes_kc, kotlinc $classes_kotlinc"
} >"$results/time-coroutines.md"
echo "wrote bench/results/time-coroutines.md"
