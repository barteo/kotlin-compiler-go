#!/usr/bin/env bash
# detekt-ab.sh compares variants of detekt on kc (jvm/kc-detekt): each
# variant is a libkc.so and JVM options, run RUNS times, round robin so
# that drift on the machine spreads over all of them. It prints each
# variant's median and minimum wall time and detekt's CreateSettings and
# Analyzer phases, and checks that every run's findings are the first
# variant's.
#
#	DETEKT_HOME=... RUNS=7 bench/detekt-ab.sh out-dir 'name|libkc.so|jvm options' ... -- detekt arguments
#
# For example, with a libkc.so built from the base commit:
#
#	bench/detekt-ab.sh /tmp/ab 'base|/tmp/libkc-base.so|-Dkc.eager=false -Dkc.cache=false' \
#		'new|jvm/kc-detekt/build/libkc.so|' -- -i src --analysis-mode full --all-rules -cp ...
#
# jvm/kc-detekt/build.sh must have run. A variant's options may name
# -XX:SharedArchiveFile: its first run, which writes the archive, is
# discarded.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
: "${DETEKT_HOME:?set DETEKT_HOME to a detekt-cli distribution}"
runs=${RUNS:-7}
out=$1
shift
variants=()
while [ "$1" != "--" ]; do
	variants+=("$1")
	shift
done
shift
mkdir -p "$out"
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}
cp=$(sed -n 's/^CLASSPATH=//p' "$DETEKT_HOME/bin/detekt-cli" | tr ':' '\n' |
	grep -v 'detekt-kotlin-analysis-api-standalone' | sed "s|\$APP_HOME|$DETEKT_HOME|" | paste -sd: -)
jar=$root/jvm/kc-detekt/build/kc-detekt.jar

run() { # name lib opts index detekt-arguments...
	local log=$out/$1.$4 lib=$2 opts=$3
	shift 4
	local start end
	start=$(date +%s%N)
	# shellcheck disable=SC2086 # the options are words
	LD_PRELOAD=$java_home/lib/libjsig.so java $opts -Dkc.lib="$lib" -cp "$jar:$cp" dev.detekt.cli.Main "$@" \
		--debug -r "checkstyle:$log.xml" >"$log.out" 2>"$log.err" || true
	end=$(date +%s%N)
	echo $(((end - start) / 1000000)) >"$log.ms"
}

# Archives first.
for v in "${variants[@]}"; do
	IFS='|' read -r name lib opts <<<"$v"
	case $opts in *SharedArchiveFile*) run "$name" "$lib" "$opts" warm "${@}" ;; esac
done
for i in $(seq 1 "$runs"); do
	for v in "${variants[@]}"; do
		IFS='|' read -r name lib opts <<<"$v"
		run "$name" "$lib" "$opts" "$i" "${@}"
	done
done

# findings normalizes a checkstyle report to file:line:column:rule lines.
findings() {
	python3 -c 'import sys, xml.etree.ElementTree as ET
t = ET.parse(sys.argv[1]).getroot()
for f in t.iter("file"):
    for e in f.iter("error"):
        print(":".join((f.get("name"), e.get("line"), e.get("column"), e.get("source"))))' "$1" | sort
}
first=${variants[0]%%|*}
findings "$out/$first.1.xml" >"$out/reference.txt"
printf '%-12s %8s %8s %16s %10s %s\n' variant median min CreateSettings Analyzer findings
for v in "${variants[@]}"; do
	name=${v%%|*}
	same=yes
	for i in $(seq 1 "$runs"); do
		findings "$out/$name.$i.xml" | cmp -s - "$out/reference.txt" || same="NO (run $i)"
	done
	phase() { # phase name → median seconds
		for i in $(seq 1 "$runs"); do
			sed -n "s/^Phase $1 took //p" "$out/$name.$i.out" |
				python3 -c 'import sys, re
s = sys.stdin.read().strip(); m = re.match(r"([\d.]+)(ms|s|us)", s)
print(float(m[1]) / {"ms": 1000, "s": 1, "us": 1e6}[m[2]] if m else 0)'
		done | sort -n | awk '{a[NR] = $1} END {printf "%.2f", a[int((NR + 1) / 2)]}'
	}
	ms=$(cat "$out/$name".[0-9]*.ms | sort -n)
	median=$(echo "$ms" | awk '{a[NR] = $1} END {printf "%.2f", a[int((NR + 1) / 2)] / 1000}')
	min=$(echo "$ms" | head -1 | awk '{printf "%.2f", $1 / 1000}')
	printf '%-12s %7ss %7ss %15ss %9ss %s (%d)\n' "$name" "$median" "$min" "$(phase CreateSettings)" "$(phase Analyzer)" \
		"$same" "$(wc -l <"$out/reference.txt")"
done
