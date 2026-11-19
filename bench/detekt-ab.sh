#!/usr/bin/env bash
# detekt-ab.sh compares variants of detekt on kc (jvm/detekt): each variant
# is a libkc.so, JVM options and optionally a kc-aa.jar and a detekt.jar
# (by default jvm/kc-aa/build's and jvm/detekt/build's), run RUNS times, round robin so that drift on the
# machine spreads over all of them. It prints each variant's median and
# minimum wall time and detekt's CreateSettings, ValidateClasspath and
# Analyzer phases, and checks that every run's findings are the first
# variant's.
#
#	RUNS=7 bench/detekt-ab.sh out-dir 'name|libkc.so|jvm options[|kc-aa.jar[|detekt.jar]]' ... -- detekt arguments
#
# For example, against a build of the base commit:
#
#	bench/detekt-ab.sh /tmp/ab 'base|/tmp/base/libkc.so||/tmp/base/kc-aa.jar' \
#		'new|jvm/kc-aa/build/libkc.so|' -- -i src --analysis-mode full --all-rules --parallel -cp ...
#
# jvm/detekt/build.sh must have run. Unlike jvm/detekt/run.sh, it adds
# neither --parallel nor a class-data sharing archive: a variant's options
# may name -XX:SharedArchiveFile, and then its first run, which writes the
# archive, is discarded.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
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
# detekt's class path, as jvm/detekt/build.sh wrote it, without kc-aa.jar
# and detekt.jar.
cp=$(tr ':' '\n' <"$root/jvm/detekt/build/classpath.txt" | grep -v '/kc-aa\.jar$\|/detekt\.jar$' | paste -sd: -)
jar=$root/jvm/kc-aa/build/kc-aa.jar
detekt=$root/jvm/detekt/build/detekt.jar

run() { # name lib opts jar detekt-jar index detekt-arguments...
	local log=$out/$1.$6 lib=$2 opts=$3 jar=${4:-$jar} detekt=${5:-$detekt}
	shift 6
	local start end
	start=$(date +%s%N)
	# shellcheck disable=SC2086 # the options are words
	LD_PRELOAD=$java_home/lib/libjsig.so java -Xss8m $opts -Dkc.lib="$lib" -Didea.home.path="$root/jvm/detekt/build" \
		-cp "$detekt:$jar:$cp" dev.detekt.cli.Main "$@" \
		--debug -r "checkstyle:$log.xml" >"$log.out" 2>"$log.err" || true
	end=$(date +%s%N)
	echo $(((end - start) / 1000000)) >"$log.ms"
}

# Archives first.
for v in "${variants[@]}"; do
	IFS='|' read -r name lib opts vjar vdetekt <<<"$v"
	case $opts in *SharedArchiveFile*) run "$name" "$lib" "$opts" "$vjar" "$vdetekt" warm "${@}" ;; esac
done
for i in $(seq 1 "$runs"); do
	for v in "${variants[@]}"; do
		IFS='|' read -r name lib opts vjar vdetekt <<<"$v"
		run "$name" "$lib" "$opts" "$vjar" "$vdetekt" "$i" "${@}"
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
printf '%-12s %8s %8s %16s %18s %10s %s\n' variant median min CreateSettings ValidateClasspath Analyzer findings
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
	printf '%-12s %7ss %7ss %15ss %17ss %9ss %s (%d)\n' "$name" "$median" "$min" "$(phase CreateSettings)" \
		"$(phase ValidateClasspath)" "$(phase Analyzer)" \
		"$same" "$(wc -l <"$out/reference.txt")"
done
