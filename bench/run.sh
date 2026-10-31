#!/bin/sh
# Benchmarks kc against kotlinc on generated projects.
#
#   bench/run.sh                                        # quick: 200 files, about 1 minute
#   SIZES="100 1000" RUNS=1 WARM_RUNS=4 bench/run.sh    # thorough: about 6 minutes
#
# SIZES: project sizes in files. RUNS: cold kotlinc runs per size (kc
# always gets 5). WARM_RUNS: kotlinc runs in one JVM; the median of the
# second half is reported. HEAP: kotlinc's maximum heap (default 8g).
#
# Needs: go, java (JDK 21), kotlinc, hyperfine. Results go to bench/results/.
# COOLDOWN: seconds to idle before each run, outside the measured time (default 0).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
work="$TMPDIR/kc-bench"
results="$root/bench/results"
sizes=${SIZES:-200}
runs=${RUNS:-2}
cooldown=${COOLDOWN:-0}
warm_runs=${WARM_RUNS:-5}
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
# kotlinc always reads the Kotlin standard library; give kc the same jar.
stdlib="$kotlin_home/lib/kotlin-stdlib.jar"
cpus=$(getconf _NPROCESSORS_ONLN)
# kotlinc's launcher defaults to -Xmx512M, too small for the larger projects.
# HEAP: kotlinc's maximum heap; at 1,000 files, 4g leaves it mostly
# collecting garbage.
export JAVA_OPTS="-Xmx${HEAP:-8g}"
# Homebrew's kotlinc otherwise picks the newest JDK; use the same one everywhere.
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
export PATH="$JAVA_HOME/bin:$PATH"

mkdir -p "$work" "$results"
(cd "$root" && go build -o "$work/kc" ./cmd/kc && go build -o "$work/gen" ./bench/gen)
mkdir -p "$work/warm"
javac -d "$work/warm" "$root/bench/warm/WarmKotlinc.java"

summary="$results/summary-$(echo $sizes | tr ' ' '-').md"
{
	echo "# kc vs kotlinc"
	echo
	echo "- Machine: $(uname -sm), $cpus CPUs"
	echo "- $(go version)"
	echo "- $(java -version 2>&1 | grep -v JAVA_TOOL | head -1)"
	echo "- $(kotlinc -version 2>&1 | grep -v JAVA_TOOL | tail -1)"
	echo "- kotlinc JVM options: $JAVA_OPTS"
	echo "- Cooldown before each run (outside timing): $cooldown s"
	echo
} >"$summary"

for n in $sizes; do
	src="$work/src$n"
	rm -rf "$src"
	"$work/gen" -n "$n" -o "$src"
	lines=$(cat "$src"/*.kt | wc -l | tr -d ' ')
	echo "== $n files, $lines lines"

	# kc takes at most a few seconds and varies little, so 5 runs are enough.
	hyperfine --warmup 1 --runs 5 --style basic \
		--export-json "$work/kc-$n.json" \
		--prepare "rm -rf $work/out-kc && sleep $cooldown" \
		-n "kc -j 1" "$work/kc -j 1 -cp $stdlib -d $work/out-kc $src" \
		-n "kc -j $cpus" "$work/kc -j $cpus -cp $stdlib -d $work/out-kc $src"
	hyperfine --runs "$runs" --style basic \
		--export-json "$work/kotlinc-$n.json" \
		--prepare "rm -rf $work/out-kotlinc && sleep $cooldown" \
		-n "kotlinc (cold JVM)" "kotlinc -nowarn -d $work/out-kotlinc $src"
	python3 -c 'import json, sys
r = [x for f in sys.argv[2:] for x in json.load(open(f))["results"]]
json.dump({"results": r}, open(sys.argv[1], "w"), indent=2)' \
		"$results/hyperfine-$n.json" "$work/kc-$n.json" "$work/kotlinc-$n.json"

	iters=$warm_runs
	warm=$(java $JAVA_OPTS -Dcooldown="$cooldown" -cp "$work/warm:$kotlin_home/lib/*" WarmKotlinc "$iters" \
		-nowarn -kotlin-home "$kotlin_home" -d "$work/out-warm" "$src" 2>/dev/null | tee "$results/warm-$n.txt" | tail -1)
	echo "kotlinc (warm JVM): $warm"

	# Both compilers' output must print the same checksum.
	sum_kc=$(java -cp "$work/out-kc:$stdlib" MainKt 2>/dev/null)
	sum_kotlinc=$(kotlin -cp "$work/out-kotlinc" MainKt 2>/dev/null)
	same=yes
	[ "$sum_kc" = "$sum_kotlinc" ] || same="NO ($sum_kc vs $sum_kotlinc)"

	{
		echo "## $n files ($lines lines)"
		echo
		python3 "$root/bench/summarize.py" "$results/hyperfine-$n.json" "kc -j 1" "kc -j $cpus" "kotlinc (cold JVM)"
		echo
		echo "- kotlinc in a warm JVM (median of the last $((iters - iters / 2)) of $iters runs in one JVM): ${warm#warm median: }"
		echo "- Same program output: $same ($sum_kc)"
		echo
	} >>"$summary"
done
echo "wrote $summary"
