#!/bin/sh
# Shows where warm kotlinc spends its time on a generated project, next to
# kc's phases on the same input.
#
#   bench/profile.sh 200                               # quick: 200 files, about 30 s
#   ITERS=10 BACKEND_THREADS=1 bench/profile.sh        # thorough: 500 files, about 12 minutes
#
# ITERS: kotlinc runs in one JVM; phase times are the median of the second
# half, which is also what JFR records. BACKEND_THREADS=1 also times
# kotlinc with -Xbackend-threads=0. HEAP: kotlinc's maximum heap (default
# 8g).
#
# Needs the same tools as bench/run.sh. Writes bench/results/phases-N.md.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
n=${1:-500}
work="$TMPDIR/kc-bench"
out="$work/profile$n"
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
stdlib="$kotlin_home/lib/kotlin-stdlib.jar"
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
export PATH="$JAVA_HOME/bin:$PATH"
iters=${ITERS:-6}
report="$root/bench/results/phases-$n.md"

rm -rf "$out"
mkdir -p "$out/perf" "$out/jfr" "$work/warm"
(cd "$root" && go build -o "$work/kc" ./cmd/kc && go build -o "$work/gen" ./bench/gen)
javac -d "$work/warm" "$root/bench/warm/WarmKotlinc.java"
"$work/gen" -n "$n" -o "$out/src"
# warm [jvm options...] -- [kotlinc options...]: prints the warm median.
warm() {
	jvm=""
	while [ "$1" != "--" ]; do jvm="$jvm $1"; shift; done
	shift
	# shellcheck disable=SC2086
	java -Xmx"${HEAP:-8g}" $jvm -cp "$work/warm:$kotlin_home/lib/*" WarmKotlinc "$iters" \
		-nowarn "$@" -kotlin-home "$kotlin_home" -d "$out/classes" "$out/src" 2>/dev/null | tail -1
}

# One JVM records the per-phase stats of every run and profiles the second
# half with JFR. Profiling adds a little to those runs' times.
# JFR's default repository may not be writable (e.g. in a sandbox), and its
# default stack depth of 64 cuts off the frames that identify the phase.
echo "kotlinc warm: $iters runs in one JVM, with per-phase stats and JFR"
warm_plain=$(warm -XX:FlightRecorderOptions:repository="$out/jfr",stackdepth=2048 -Djfr="$out/kotlinc.jfr" -- \
	-Xdetailed-perf "-Xdump-perf=$out/perf/{run}.json")
jfr print --stack-depth 2048 --events jdk.ExecutionSample "$out/kotlinc.jfr" >"$out/samples.txt" 2>/dev/null

backend=""
if [ "${BACKEND_THREADS:-0}" = 1 ]; then
	echo "kotlinc warm, with -Xbackend-threads=0"
	warm_bt=$(warm -- -Xbackend-threads=0)
	backend="; with \`-Xbackend-threads=0\`: ${warm_bt#warm median: }"
fi

echo "kc phases (median of 5 runs)"
kc_phases() {
	for i in 1 2 3 4 5; do "$work/kc" -time -j "$1" -cp "$stdlib" -d "$out/kc" "$out/src" 2>&1; done |
		python3 -c '
import re, statistics, sys
t = {}
for line in sys.stdin:
    name, d = line.split()[:2]
    v = float(re.match(r"[\d.]+", d).group())
    v = v / 1000 if d.endswith("µs") else v * 1000 if not d.endswith("ms") else v
    t.setdefault(name, []).append(v)
print(" | ".join(f"{k} {statistics.median(v):,.1f} ms" for k, v in t.items()))'
}

{
	echo "# Where kotlinc's time goes: $n files"
	echo
	echo "- $(java -version 2>&1 | grep -v JAVA_TOOL | head -1), $(kotlinc -version 2>&1 | grep -v JAVA_TOOL | tail -1)"
	echo "- kotlinc warm, profiled: ${warm_plain#warm median: }$backend (profiling slows kotlinc down;"
	echo "  bench/run.sh measures it without, and the phase times below include it)"
	echo "- kc -j 1: $(kc_phases 1)"
	echo "- kc -j $(getconf _NPROCESSORS_ONLN): $(kc_phases "$(getconf _NPROCESSORS_ONLN)")"
	echo
	echo "## kotlinc phases (\`-Xdump-perf\`)"
	echo
	python3 "$root/bench/phases.py" "$out/perf"
	echo
	echo "## kotlinc profile (JFR)"
	echo
	python3 "$root/bench/profile.py" "$out/samples.txt"
} >"$report"
echo "wrote $report"
