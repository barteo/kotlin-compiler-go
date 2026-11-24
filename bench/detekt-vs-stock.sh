#!/usr/bin/env bash
# detekt-vs-stock.sh times stock detekt-cli with --parallel against detekt
# on kc (jvm/detekt/run.sh, which adds --parallel and a class-data sharing
# archive), round robin, and prints each one's median and minimum wall
# time, the speedup of the medians and whether kc's findings were the same
# in every run.
#
#	STOCK_JAR=detekt-cli-2.0.0-alpha.6-all.jar RUNS=5 bench/detekt-vs-stock.sh out-dir -- detekt arguments
#
# The arguments are detekt-cli's, the same for both (--analysis-mode full
# --all-rules -cp ... --jdk-home ...); compiler options such as -opt-in go
# last. A first run of each, which writes kc's archive and warms the page
# cache, is discarded. KC_JAVA_OPTS adds JVM options to kc's runs.
#
# STOCK_FRAGMENTS=common=DIR;jvm=DIR:common runs stock with a source module
# per fragment, each depending on the fragments it refines, as Gradle's
# multiplatform compilations do (bench/detekt-stock-mpp). Plain detekt-cli
# puts all the roots in one module, where an expect declaration and its
# actual one are redeclarations, which leaves much of their code unresolved.
# KOTLINC compiles the patch (default: kotlinc on PATH).
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
runs=${RUNS:-5}
out=$1
shift
[ "$1" = -- ] && shift
stock=${STOCK_JAR:?STOCK_JAR names detekt-cli-*-all.jar}
mkdir -p "$out"
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}
stock_cmd=(-jar "$stock")
if [ -n "${STOCK_FRAGMENTS:-}" ]; then
	patch=$out/stock-mpp
	if [ ! -f "$patch/dev/detekt/core/settings/EnvironmentFacade.class" ]; then
		"${KOTLINC:-kotlinc}" -no-stdlib -no-reflect -cp "$stock" -jvm-target 17 -api-version 2.2 -language-version 2.2 \
			-opt-in=org.jetbrains.kotlin.analysis.api.KaExperimentalApi -nowarn \
			-d "$patch" "$root/bench/detekt-stock-mpp/EnvironmentAware.kt"
	fi
	stock_cmd=("-Ddetekt.fragments=$STOCK_FRAGMENTS" -cp "$patch:$stock" dev.detekt.cli.Main)
fi

run() { # name index
	local log=$out/$1.$2 start end
	start=$(date +%s%N)
	if [ "$1" = stock ]; then
		"$java_home/bin/java" -Xss8m "${stock_cmd[@]}" --parallel "${args[@]}" -r "checkstyle:$log.xml" >"$log.out" 2>"$log.err" || true
	else
		"$root/jvm/detekt/run.sh" "${args[@]}" -r "checkstyle:$log.xml" >"$log.out" 2>"$log.err" || true
	fi
	end=$(date +%s%N)
	echo $(((end - start) / 1000000)) >"$log.ms"
}
args=("$@")

run stock warm
run kc warm
for i in $(seq 1 "$runs"); do
	run stock "$i"
	run kc "$i"
done

findings() {
	python3 -c 'import sys, xml.etree.ElementTree as ET
t = ET.parse(sys.argv[1]).getroot()
for f in t.iter("file"):
    for e in f.iter("error"):
        print(":".join((f.get("name"), e.get("line"), e.get("column"), e.get("source"))))' "$1" | sort
}
stat() { # name → "median min max" in seconds
	cat "$out/$1".[0-9]*.ms | sort -n | awk '{a[NR] = $1} END {printf "%.2f %.2f %.2f", a[int((NR + 1) / 2)] / 1000, a[1] / 1000, a[NR] / 1000}'
}
findings "$out/kc.1.xml" >"$out/kc.txt"
findings "$out/stock.1.xml" >"$out/stock.txt"
same=yes
for i in $(seq 1 "$runs"); do
	findings "$out/kc.$i.xml" | cmp -s - "$out/kc.txt" || same="NO (run $i)"
done
read -r sm smin smax <<<"$(stat stock)"
read -r km kmin kmax <<<"$(stat kc)"
printf '%-6s %8s %8s %8s %s\n' "" median min max findings
printf '%-6s %7ss %7ss %7ss %d\n' stock "$sm" "$smin" "$smax" "$(wc -l <"$out/stock.txt")"
printf '%-6s %7ss %7ss %7ss %d, the same in every run: %s, matching stock: %d\n' kc "$km" "$kmin" "$kmax" \
	"$(wc -l <"$out/kc.txt")" "$same" "$(comm -12 "$out/kc.txt" "$out/stock.txt" | wc -l)"
python3 -c 'import sys; print("speedup %.2fx" % (float(sys.argv[1]) / float(sys.argv[2])))' "$sm" "$km"
