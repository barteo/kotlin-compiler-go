#!/bin/sh
# Runs JetBrains' "ring" micro-benchmarks, the main group of Kotlin's own
# performance suite (kotlin-native/performance/ring in JetBrains/kotlin),
# compiled by kc and by kotlinc, under JMH on the same JVM, and writes
# bench/results/ring.md. This times the code each compiler writes, not
# compiling.
#
#   bench/ring.sh                               # all 200 benchmarks, about 15 minutes
#   FILTER='ring\.(Inline|Lambda)\.' bench/ring.sh  # a JMH regexp: some of them
#   FORKS=3 bench/ring.sh                       # more JVMs per benchmark
#
# The suite is written for Kotlin/Native and run there with
# kotlinx-benchmark. Its common sources compile for the JVM too; this
# script:
#
# - checks out kotlin-native/performance at REV;
# - leaves out the files that use Kotlin/Native's APIs (workers, weak
#   references, atomics, getFloatAt);
# - gives the files without a package one, which JMH requires;
# - replaces the adapter's SkipWhenBaseOnly with bench/ring/SkipWhenBaseOnly.kt;
# - compiles the rest with each compiler, against kotlinx-benchmark's JVM
#   runtime, whose annotations are JMH's;
# - opens the @State classes (bench/ring/OpenStateClasses.java), as
#   kotlinx-benchmark's Gradle plugin does with all-open;
# - generates JMH's harness from each compiler's class files and runs it.
#
# FORKS: JMH forks per benchmark and build, alternating between the builds
# (default 1). WARMUP and ITERS: warmup and measured iterations of TIME
# each (default 5, 5 and 200ms). JVM_OPTS: the forks' JVM options (default
# a fixed 1 GB heap). REV: the JetBrains/kotlin commit.
#
# Needs: go, java (JDK 21), kotlinc, git, curl, python3.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
work="$TMPDIR/kc-ring"
results="$root/bench/results"
rev=${REV:-c2b1102ad6118f0767e1b576cbd9f35e5b2565da}
forks=${FORKS:-1}
warmup=${WARMUP:-5}
iters=${ITERS:-5}
time=${TIME:-200ms}
filter=${FILTER:-}
jvm_opts=${JVM_OPTS:-"-Xms1g -Xmx1g"}
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
stdlib="$kotlin_home/lib/kotlin-stdlib.jar"
brew_jdk=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home
if [ -z "${JAVA_HOME:-}" ] && [ -d "$brew_jdk" ]; then
	export JAVA_HOME="$brew_jdk"
fi
[ -n "${JAVA_HOME:-}" ] && export PATH="$JAVA_HOME/bin:$PATH"
[ -f "$stdlib" ] || { echo "no $stdlib: set KOTLIN_HOME to kotlinc's home"; exit 1; }

# JMH, its bytecode generator and kotlinx-benchmark's JVM runtime, from
# Maven Central, kept between runs.
jars="$work/jars"
mkdir -p "$jars"
while read -r sum path; do
	f="$jars/${path##*/}"
	if [ ! -f "$f" ]; then
		curl -sSfL --retry 5 --retry-all-errors -o "$f.part" "https://repo1.maven.org/maven2/$path"
		mv "$f.part" "$f"
	fi
	echo "$sum  $f" | sha256sum -c --quiet - || { rm -f "$f"; exit 1; }
done <<'EOF'
dc0eaf2bbf0036a70b60798c785d6e03a9daf06b68b8edb0f1ba9eb3421baeb3 org/openjdk/jmh/jmh-core/1.37/jmh-core-1.37.jar
619b3d15a5e8bfc4ec49d3b7a64dbed053a0ca8625820a49a4f869d1b8c71d09 org/openjdk/jmh/jmh-generator-bytecode/1.37/jmh-generator-bytecode-1.37.jar
de29bacc5c3a413215800f57de9017fdda1b3cb6e5359ea0c84ebe13c9610222 org/openjdk/jmh/jmh-generator-asm/1.37/jmh-generator-asm-1.37.jar
a0421dbbe5e77690df2dfdef98618b62852d816bbb814c5cbd0b4d464bff32b0 org/openjdk/jmh/jmh-generator-reflection/1.37/jmh-generator-reflection-1.37.jar
8cadd43ac5eb6d09de05faecca38b917a040bb9139c7edeb4cc81c740b713281 org/ow2/asm/asm/9.7.1/asm-9.7.1.jar
df26cc58f235f477db07f753ba5a3ab243ebe5789d9f89ecf68dd62ea9a66c28 net/sf/jopt-simple/jopt-simple/5.0.4/jopt-simple-5.0.4.jar
1e56d7b058d28b65abd256b8458e3885b674c1d588fa43cd7d1cbb9c7ef2b308 org/apache/commons/commons-math3/3.6.1/commons-math3-3.6.1.jar
f44154d4f11bdca7fab7aa519e0d51df4fe3bb959f3177a42ef5603f30f49a63 org/jetbrains/kotlinx/kotlinx-benchmark-runtime-jvm/0.5.0/kotlinx-benchmark-runtime-jvm-0.5.0.jar
EOF
jmh_cp=$(ls "$jars"/*.jar | paste -sd: -)
compile_cp="$jars/kotlinx-benchmark-runtime-jvm-0.5.0.jar:$jars/jmh-core-1.37.jar"

# The suite's sources, kept between runs.
repo="$work/kotlin"
if [ "$(git -C "$repo" rev-parse HEAD 2>/dev/null)" != "$rev" ]; then
	rm -rf "$repo"
	git init -q "$repo"
	git -C "$repo" remote add origin https://github.com/JetBrains/kotlin.git
	git -C "$repo" sparse-checkout set kotlin-native/performance/ring
	git -C "$repo" fetch -q --depth 1 --filter=blob:none origin "$rev"
	git -C "$repo" checkout -q FETCH_HEAD
fi

src="$work/src"
rm -rf "$src" "$work/runs" "$work"/out-* "$work"/gen-* "$work"/harness-*
mkdir -p "$work/runs" "$results"
cp -R "$repo/kotlin-native/performance/ring/src/commonMain/kotlin/org/jetbrains/ring" "$src"
# Kotlin/Native only.
for f in DefaultRandomBenchmark LifeBenchmark LinkedListWithAtomicsBenchmark \
	SplayBenchmark WeakRefBenchmark BunnymarkBenchmark; do
	rm "$src/$f.kt"
done
for f in DeltaBlueBenchmark.kt:deltablue RichardsBenchmark.kt:richards \
	GenericArrayViewBenchmark.kt:genericarrayview \
	OctoTest/basicTest.kt:octotest OctoTest/ocTree.kt:octotest; do
	file="$src/${f%:*}"
	grep -q '^package ' "$file" && { echo "$file has a package now"; exit 1; }
	{ echo "package org.jetbrains.ring.${f#*:}"; cat "$file"; } >"$file.new"
	mv "$file.new" "$file"
done
cp "$root/bench/ring/SkipWhenBaseOnly.kt" "$src/"
files=$(find "$src" -name '*.kt' | sort)
echo "$(echo "$files" | wc -l | tr -d ' ') files, $(cat $files | wc -l | tr -d ' ') lines"

(cd "$root" && go build -o "$work/kc" ./cmd/kc)
# shellcheck disable=SC2086 # the files are words
"$work/kc" -cp "$stdlib:$compile_cp" -d "$work/out-kc" $files
# shellcheck disable=SC2086
kotlinc -nowarn -cp "$compile_cp" -d "$work/out-kotlinc" $files 2>&1 | grep -v JAVA_TOOL_OPTIONS || true
[ -d "$work/out-kotlinc/org/jetbrains/ring" ] || { echo "kotlinc failed"; exit 1; }

for c in kc kotlinc; do
	java -cp "$jars/asm-9.7.1.jar" "$root/bench/ring/OpenStateClasses.java" "$work/out-$c" 2>/dev/null
	mkdir -p "$work/gen-$c" "$work/harness-$c"
	java -cp "$jmh_cp:$stdlib:$work/out-$c" org.openjdk.jmh.generators.bytecode.JmhBytecodeGenerator \
		"$work/out-$c" "$work/gen-$c" "$work/harness-$c" asm 2>/dev/null >/dev/null
	javac -nowarn -cp "$jmh_cp:$stdlib:$work/out-$c" -d "$work/harness-$c" \
		$(find "$work/gen-$c" -name '*.java') 2>&1 | grep -v JAVA_TOOL_OPTIONS || true
	echo "$c: $(wc -l <"$work/harness-$c/META-INF/BenchmarkList" | tr -d ' ') benchmarks"
done

i=1
while [ "$i" -le "$forks" ]; do
	for c in kc kotlinc; do
		echo "== fork $i/$forks: $c"
		# shellcheck disable=SC2086 # the filter may be empty
		java -cp "$work/harness-$c:$work/out-$c:$jmh_cp:$stdlib" org.openjdk.jmh.Main \
			-f 1 -wi "$warmup" -w "$time" -i "$iters" -r "$time" -bm avgt -tu ns \
			-jvmArgs "$jvm_opts" -rf json -rff "$work/runs/$c-$i.json" $filter 2>/dev/null |
			grep '^# Benchmark:' || true
	done
	i=$((i + 1))
done

out="$results/ring.md"
{
	echo "# JetBrains' ring benchmarks: kc vs kotlinc"
	echo
	echo "The \"ring\" micro-benchmarks of Kotlin's performance suite"
	echo "([kotlin-native/performance/ring](https://github.com/JetBrains/kotlin/tree/$rev/kotlin-native/performance/ring)),"
	echo "compiled by each compiler and run under JMH on the same JVM. Generated by"
	echo "\`bench/ring.sh\`."
	echo
	echo "- Machine: $(uname -sm), $(getconf _NPROCESSORS_ONLN) CPUs"
	echo "- $(java -version 2>&1 | grep -v JAVA_TOOL | head -1)"
	echo "- $(kotlinc -version 2>&1 | grep -v JAVA_TOOL | tail -1)"
	echo "- JMH 1.37, average time: $forks fork(s) per build, $warmup warmup and $iters measured iterations of $time; JVM options: $jvm_opts"
	echo
	python3 - "$work/runs" "$forks" <<'PY'
import json, statistics, sys
runs, forks = sys.argv[1], int(sys.argv[2])
score = {}
for c in ("kc", "kotlinc"):
    for i in range(1, forks + 1):
        for r in json.load(open(f"{runs}/{c}-{i}.json")):
            name = r["benchmark"].removeprefix("org.jetbrains.ring.")
            score.setdefault((c, name), []).append(r["primaryMetric"]["score"])
names = sorted({n for _, n in score if ("kc", n) in score and ("kotlinc", n) in score})
rows = []
for n in names:
    k = statistics.mean(score[("kotlinc", n)])
    g = statistics.mean(score[("kc", n)])
    rows.append((n, k, g, g / k))
gm = statistics.geometric_mean([r[3] for r in rows])
slower = sum(r[3] > 1.1 for r in rows)
faster = sum(r[3] < 1 / 1.1 for r in rows)
print(f"{len(rows)} benchmarks. Time per operation, so lower is better; a ratio above 1 means kc's code is slower.")
print()
print(f"- **Geometric mean of kc / kotlinc: {gm:.2f}**")
print(f"- kc's code is more than 10% slower on {slower}, more than 10% faster on {faster}, and within 10% on {len(rows) - slower - faster}.")
print()

def fmt(ns):
    return f"{ns:,.1f}" if ns < 100 else f"{ns:,.0f}"

def table(rs):
    print("| Benchmark | `kotlinc` ns/op | `kc` ns/op | kc / kotlinc |")
    print("|:---|---:|---:|---:|")
    for n, k, g, r in rs:
        print(f"| {n} | {fmt(k)} | {fmt(g)} | {r:.2f} |")
    print()

groups = {}
for r in rows:
    groups.setdefault(r[0].rsplit(".", 1)[0], []).append(r[3])
print("## By class")
print()
print("Geometric mean of kc / kotlinc over each class's benchmarks.")
print()
print("| Class | Benchmarks | kc / kotlinc |")
print("|:---|---:|---:|")
for g, rs in sorted(groups.items(), key=lambda x: -statistics.geometric_mean(x[1])):
    print(f"| {g} | {len(rs)} | {statistics.geometric_mean(rs):.2f} |")
print()
by_ratio = sorted(rows, key=lambda r: -r[3])
print("## kc's slowest, relative to kotlinc")
print()
table(by_ratio[:20])
print("## kc's fastest, relative to kotlinc")
print()
table(list(reversed(by_ratio[-10:])))
print("## All benchmarks")
print()
table(rows)
PY
} >"$out"
echo "wrote $out"
sed -n '1,/^## By class/p' "$out"
