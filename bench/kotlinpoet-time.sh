#!/bin/sh
# Times compiling square/kotlinpoet's JVM library (commonMain and jvmMain)
# with kc and kotlinc, and writes bench/results/time-kotlinpoet.md.
#
#   bench/kotlinpoet-time.sh                      # about 2 minutes
#   RUNS=5 WARM_RUNS=20 bench/kotlinpoet-time.sh
#
# Both compilers get the build's options: the module name, the opt-in and
# -jvm-default=no-compatibility, and kotlin-reflect on the class path;
# kotlinc compiles commonMain and jvmMain as two fragments of a
# multiplatform module, as Gradle does, with -Xjdk-release=8 (kc always
# writes Java 8 class files). RUNS: cold kotlinc runs (kc always gets
# 10). WARM_RUNS: kotlinc runs in one JVM, of which the median of the
# second half is reported.
#
# Needs the same tools as bench/run.sh, and the clone that
# bench/survey.sh makes.
# COOLDOWN: seconds to idle before each run, outside the measured time (default 0).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
tag=${TAG:-2.4.0}
src="$TMPDIR/kc-survey/kotlinpoet-$tag/kotlinpoet/src"
work="$TMPDIR/kc-kotlinpoet-time"
results="$root/bench/results"
runs=${RUNS:-3}
cooldown=${COOLDOWN:-0}
warm_runs=${WARM_RUNS:-10}
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
stdlib="$kotlin_home/lib/kotlin-stdlib.jar"
reflect="$kotlin_home/lib/kotlin-reflect.jar"
cpus=$(getconf _NPROCESSORS_ONLN)
export JAVA_OPTS="-Xmx${HEAP:-8g}"
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
export PATH="$JAVA_HOME/bin:$PATH"

[ -d "$src" ] || { echo "no kotlinpoet clone at $src: run bench/survey.sh first"; exit 1; }
rm -rf "$work"
mkdir -p "$work/warm" "$results"
(cd "$root" && go build -o "$work/kc" ./cmd/kc)
javac -d "$work/warm" "$root/bench/warm/WarmKotlinc.java"

common=$(find "$src/commonMain" -name '*.kt' | sort)
jvm=$(find "$src/jvmMain" -name '*.kt' | sort)
files=$(echo "$common $jvm" | wc -w | tr -d ' ')
lines=$(cat $common $jvm | wc -l | tr -d ' ')
fragments=$( (echo "$common" | sed 's/^/common:/'; echo "$jvm" | sed 's/^/jvm:/') | paste -sd, -)
module=kotlinpoet
opts="-module-name $module -jvm-default=no-compatibility -opt-in=com.squareup.kotlinpoet.DelicateKotlinPoetApi"
kotlinc_args="-nowarn -Xmulti-platform -Xexpect-actual-classes -Xfragments=common,jvm -Xfragment-refines=jvm:common
	-Xfragment-sources=$fragments -Xjdk-release=8 $opts -cp $reflect"

# shellcheck disable=SC2086 # the arguments are words
hyperfine --warmup 1 --runs 10 --style basic --export-json "$work/kc.json" \
	--prepare "rm -rf $work/out-kc && sleep $cooldown" \
	-n "kc -j 1" "$work/kc -j 1 -module-name $module -cp $stdlib:$reflect -d $work/out-kc $src/commonMain $src/jvmMain" \
	-n "kc -j $cpus" "$work/kc -j $cpus -module-name $module -cp $stdlib:$reflect -d $work/out-kc $src/commonMain $src/jvmMain"
# shellcheck disable=SC2086
hyperfine --runs "$runs" --style basic --export-json "$work/kotlinc.json" \
	--prepare "rm -rf $work/out-kotlinc && sleep $cooldown" \
	-n "kotlinc (cold JVM)" "kotlinc $(echo $kotlinc_args) -d $work/out-kotlinc $common $jvm"
python3 -c 'import json, sys
r = [x for f in sys.argv[2:] for x in json.load(open(f))["results"]]
json.dump({"results": r}, open(sys.argv[1], "w"), indent=2)' \
	"$results/hyperfine-kotlinpoet.json" "$work/kc.json" "$work/kotlinc.json"

# shellcheck disable=SC2086
warm=$(java $JAVA_OPTS -Dcooldown="$cooldown" -cp "$work/warm:$kotlin_home/lib/*" WarmKotlinc "$warm_runs" \
	$kotlinc_args -kotlin-home "$kotlin_home" -d "$work/out-warm" $common $jvm 2>/dev/null |
	tee "$results/warm-kotlinpoet.txt" | tail -1)
echo "kotlinc (warm JVM): $warm"
classes_kc=$(find "$work/out-kc" -name '*.class' | wc -l | tr -d ' ')
classes_kotlinc=$(find "$work/out-kotlinc" -name '*.class' | wc -l | tr -d ' ')

{
	echo "# kc vs kotlinc on kotlinpoet"
	echo
	echo "square/kotlinpoet $tag's JVM library: $files files, $lines lines (commonMain and jvmMain)."
	echo
	echo "- Machine: $(uname -sm), $cpus CPUs"
	echo "- $(go version)"
	echo "- $(java -version 2>&1 | grep -v JAVA_TOOL | head -1)"
	echo "- $(kotlinc -version 2>&1 | grep -v JAVA_TOOL | tail -1)"
	echo "- kotlinc JVM options: $JAVA_OPTS"
	echo "- Cooldown before each run (outside timing): $cooldown s"
	echo
	python3 "$root/bench/summarize.py" "$results/hyperfine-kotlinpoet.json" "kc -j 1" "kc -j $cpus" "kotlinc (cold JVM)"
	echo
	echo "- kotlinc in a warm JVM (median of the last $((warm_runs - warm_runs / 2)) of $warm_runs runs in one JVM): ${warm#warm median: }"
	echo "- Class files: kc $classes_kc, kotlinc $classes_kotlinc"
} >"$results/time-kotlinpoet.md"
echo "wrote bench/results/time-kotlinpoet.md"
