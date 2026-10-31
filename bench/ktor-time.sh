#!/bin/sh
# Times the survey's Ktor JVM modules, each against the same compiler's
# preceding modules: ktor-io, ktor-utils, then ktor-http.
#
#   bench/ktor-time.sh
#   COOLDOWN=10 RUNS=5 WARM_RUNS=20 bench/ktor-time.sh
#
# Needs the tools used by bench/run.sh, bench/survey.sh's Ktor clone,
# and network access to Maven Central on the first run for dependencies.
# RUNS: cold builds (10 kc builds). WARM_RUNS: complete builds in one JVM;
# the median of the second half is reported. COOLDOWN: seconds idle before
# each complete build, outside timing (default 0). No Gradle tasks, tests
# or AtomicFU bytecode transformation are included in these compile times.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
tag=${TAG:-3.6.0}
src="$TMPDIR/kc-survey/ktor-$tag"
deps="$TMPDIR/kc-ktor-deps"
work="$TMPDIR/kc-ktor-time"
results="$root/bench/results"
runs=${RUNS:-3}
cooldown=${COOLDOWN:-0}
warm_runs=${WARM_RUNS:-10}
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
cpus=$(getconf _NPROCESSORS_ONLN)
export JAVA_OPTS="-Xmx${HEAP:-8g}"
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
export PATH="$JAVA_HOME/bin:$PATH"

[ -d "$src" ] || { echo "no Ktor clone at $src: run bench/survey.sh first"; exit 1; }
mkdir -p "$deps"
cp="$kotlin_home/lib/kotlin-stdlib.jar"
for jar in org/jetbrains/kotlinx/atomicfu-jvm/0.33.0/atomicfu-jvm-0.33.0.jar \
	org/jetbrains/kotlinx/kotlinx-coroutines-core-jvm/1.11.0/kotlinx-coroutines-core-jvm-1.11.0.jar \
	org/jetbrains/kotlinx/kotlinx-io-core-jvm/0.9.1/kotlinx-io-core-jvm-0.9.1.jar \
	org/jetbrains/kotlinx/kotlinx-io-bytestring-jvm/0.9.1/kotlinx-io-bytestring-jvm-0.9.1.jar \
	org/jetbrains/kotlinx/kotlinx-serialization-core-jvm/1.11.0/kotlinx-serialization-core-jvm-1.11.0.jar \
	org/slf4j/slf4j-api/2.0.19/slf4j-api-2.0.19.jar; do
	f="$deps/$(basename "$jar")"
	[ -f "$f" ] || curl -fsSL -o "$f" "https://repo1.maven.org/maven2/$jar"
	cp="$cp:$f"
done
rm -rf "$work"
mkdir -p "$work/warm" "$results"
(cd "$root" && go build -o "$work/kc" ./cmd/kc)
javac -d "$work/warm" "$root/bench/warm/WarmKotlinc.java"

# Argument files preserve the fragment/source mapping and keep commands
# short. Each compiler uses its own outputs for downstream dependencies.
python3 - "$src" "$work" "$cp" "$kotlin_home" <<'PY'
import json
from pathlib import Path
import shlex
import sys

src, work = map(Path, sys.argv[1:3])
cp, kotlin_home = sys.argv[3:5]
modules = ['ktor-io', 'ktor-utils', 'ktor-http']
files = []
commands = {kind: [] for kind in ['kc', 'kotlinc']}
for kind in ['kc', 'kotlinc', 'warm']:
    classpath = cp
    for module in modules:
        sets = ['common', 'jvmAndPosix', 'jvm'] if module == 'ktor-io' else ['common', 'jvm']
        sources = {s: sorted((src / module / s / 'src').rglob('*.kt')) for s in sets}
        all_sources = [str(p) for paths in sources.values() for p in paths]
        dest = work / ('out-' + kind) / module
        opts = ['-module-name', module, '-jvm-default=enable', '-cp', classpath, '-d', str(dest)]
        if kind == 'kc':
            dirs = [str(src / module / s / 'src') for s in sets]
            commands[kind].append([str(work / 'kc'), '-j', '"$1"', '-serialization', *opts, *dirs])
            files += [Path(p) for p in all_sources]
        else:
            refines = [f'{sets[i]}:{sets[i-1]}' for i in range(1, len(sets))]
            fragments = [f'{s}:{p}' for s, paths in sources.items() for p in paths]
            args = ['-nowarn', '-Xmulti-platform', '-Xexpect-actual-classes',
                    '-Xfragments=' + ','.join(sets), '-Xfragment-refines=' + ','.join(refines),
                    '-Xfragment-sources=' + ','.join(fragments), '-language-version', '2.3',
                    '-api-version', '2.2', '-Xexplicit-api=strict', '-jvm-target', '1.8',
                    '-kotlin-home', kotlin_home,
                    '-Xplugin=' + kotlin_home + '/lib/kotlin-serialization-compiler-plugin.jar',
                    *opts, *all_sources]
            argfile = work / f'{module}.{kind}.args'
            argfile.write_text('\n'.join(json.dumps(arg) for arg in args) + '\n')
            if kind == 'kotlinc':
                commands[kind].append(['kotlinc', '@' + str(argfile)])
        classpath += ':' + str(dest)
for kind, builds in commands.items():
    lines = ['#!/bin/sh', 'set -eu']
    for command in builds:
        lines.append(' '.join(arg if arg == '"$1"' else shlex.quote(arg) for arg in command))
    runner = work / f'run-{kind}.sh'
    runner.write_text('\n'.join(lines) + '\n')
    runner.chmod(0o755)
(work / 'sources.txt').write_text(f'{len(files)} {sum(len(p.read_bytes().splitlines()) for p in files)}\n')
PY

# shellcheck disable=SC2086 # the Java heap options are words
hyperfine --warmup 1 --runs 10 --style basic --export-json "$work/kc.json" \
	--prepare "rm -rf '$work/out-kc' && sleep $cooldown" \
	-n "kc -j 1" "'$work/run-kc.sh' 1" \
	-n "kc -j $cpus" "'$work/run-kc.sh' $cpus"
hyperfine --runs "$runs" --style basic --export-json "$work/kotlinc.json" \
	--prepare "rm -rf '$work/out-kotlinc' && sleep $cooldown" \
	-n "kotlinc (cold JVM)" "'$work/run-kotlinc.sh'"
python3 -c 'import json, sys
r = [x for f in sys.argv[2:] for x in json.load(open(f))["results"]]
json.dump({"results": r}, open(sys.argv[1], "w"), indent=2)' \
	"$results/hyperfine-ktor.json" "$work/kc.json" "$work/kotlinc.json"

# Capture the Java exit status before tee/tail, so failed compilations
# cannot be mistaken for a successful benchmark.
# shellcheck disable=SC2086
if ! java $JAVA_OPTS -Dcooldown="$cooldown" -cp "$work/warm:$kotlin_home/lib/*" WarmKotlinc "$warm_runs" \
	"@$work/ktor-io.warm.args" --next-module "@$work/ktor-utils.warm.args" \
	--next-module "@$work/ktor-http.warm.args" > "$results/warm-ktor.txt" 2> "$work/warm.log"; then
	cat "$work/warm.log" >&2
	exit 1
fi
cat "$results/warm-ktor.txt"
warm=$(tail -1 "$results/warm-ktor.txt")
read -r files lines < "$work/sources.txt"
classes_kc=$(find "$work/out-kc" -name '*.class' | wc -l | tr -d ' ')
classes_kotlinc=$(find "$work/out-kotlinc" -name '*.class' | wc -l | tr -d ' ')

{
	echo "# kc vs kotlinc on Ktor"
	echo
	echo "Ktor $tag's ktor-io, ktor-utils and ktor-http JVM libraries: $files files, $lines lines."
	echo "Modules compile sequentially against the same compiler's preceding outputs."
	echo "Cold kotlinc starts one JVM per module; warm kotlinc runs all three in one JVM."
	echo "These are compiler timings, excluding Gradle, tests and AtomicFU bytecode transformation."
	echo
	echo "- Machine: $(uname -sm), $cpus CPUs"
	echo "- Revision: $(git -C "$root" rev-parse --short HEAD)"
	echo "- $(go version)"
	echo "- $(java -version 2>&1 | grep -v JAVA_TOOL | head -1)"
	echo "- $(kotlinc -version 2>&1 | grep -v JAVA_TOOL | tail -1)"
	echo "- kotlinc JVM options: $JAVA_OPTS"
	echo "- Runs: 10 kc builds (plus one warm-up per command), $runs cold kotlinc builds, $warm_runs builds in one JVM"
	echo "- Cooldown before each complete build (outside timing): $cooldown s"
	echo "- Options: -jvm-default=enable, kotlinx.serialization; kotlinc language 2.3, API 2.2, explicit API mode, JVM target 1.8 and multiplatform fragments"
	echo "- Dependencies: atomicfu 0.33.0, coroutines 1.11.0, kotlinx-io 0.9.1, serialization 1.11.0, SLF4J 2.0.19"
	echo
	python3 "$root/bench/summarize.py" "$results/hyperfine-ktor.json" "kc -j 1" "kc -j $cpus" "kotlinc (cold JVM)"
	echo
	echo "- kotlinc in a warm JVM (median of the last $((warm_runs - warm_runs / 2)) of $warm_runs complete builds): ${warm#warm median: }"
	echo "- Class files: kc $classes_kc, kotlinc $classes_kotlinc"
} > "$results/time-ktor.md"
echo "wrote bench/results/time-ktor.md"
