#!/usr/bin/env bash
# run.sh times Room's KSP processor (room-compiler 2.8.4) with
# bench/ksp/run.sh, on stock KSP2 and on the KSP fork on kc.
#
#	bench/ksp/room/run.sh [RUNS]
#
# WORKLOAD picks the sources: "small" is jvm/kc-aa/test/room/src (one file,
# two entities, two DAOs); "large" (the default) is gen.sh's module of N
# (default 100) files, written to data/large-N. Before timing, it runs
# Room once on each backend (the build.sh test's RunKsp) and fails unless
# both generate the same files (CHECK=0 skips this). EDIT=auto edits
# r/Item000.kt (large) or r/Db.kt (small). The rest of the environment is
# bench/ksp/run.sh's (BACKENDS, COLD, JFR, KC_TIMING, JVM_OPTS). It needs
# jvm/kc-aa/build.sh stock to have run, which fetches Room's jars.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
kcaa=$root/jvm/kc-aa
out=$kcaa/build
lib=$root/jvm/ksp/build/aa/m2
export JAVA_TOOL_OPTIONS=
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}

. "$kcaa/room-jars.sh"
# paths GROUP:ARTIFACT:VERSION... prints the jars build.sh fetched, one per line.
paths() {
	local c g a v
	for c; do
		IFS=: read -r g a v <<< "$c"
		[ -s "$out/m2/$a-$v.jar" ] || { echo "run.sh: no $a-$v.jar: run jvm/kc-aa/build.sh stock first" >&2; exit 1; }
		echo "$out/m2/$a-$v.jar"
	done
}

work=$out/room-bench
mkdir -p "$work"
case ${WORKLOAD:-large} in
small)
	src=$kcaa/test/room/src
	edit=r/Db.kt
	;;
large)
	n=${N:-100}
	src=$here/data/large-$n
	[ -s "$src/r/AppDb.kt" ] && [ "$src/r/AppDb.kt" -nt "$here/gen.sh" ] || "$here/gen.sh" "$src" "$n"
	edit=r/Item000.kt
	;;
*) echo "run.sh: WORKLOAD is small or large" >&2; exit 2 ;;
esac
{ echo "$lib/kotlin-stdlib-2.4.21.jar"; paths "${room_libs[@]}"; } > "$work/libs.txt"
paths "${room_procs[@]}" > "$work/procs.txt"
STOCK_PROCS_EXTRA=$(paths "${room_serialization[@]}" | paste -sd:)
export STOCK_PROCS_EXTRA
[ "${EDIT:-}" = auto ] && export EDIT=$edit

if [ "${CHECK:-1}" != 0 ]; then
	kspwork=$root/jvm/ksp/build
	m2=$kspwork/m2
	runtime=$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar
	deps=$kspwork/aa/kc-aa-api-2.4.21.jar:$runtime:$lib/caffeine-2.9.3.jar:$kspwork/ksp-aa-kc.jar
	deps=$deps:$m2/symbol-processing-api-2.3.9.jar:$m2/symbol-processing-common-deps-2.3.9.jar
	deps=$deps:$m2/kotlinx-serialization-core-jvm-1.6.3.jar:$m2/kotlinx-serialization-json-jvm-1.6.3.jar
	stockcp=
	for jar in symbol-processing-aa-embeddable-2.3.6 symbol-processing-api-2.3.6 symbol-processing-common-deps-2.3.6 \
		kotlin-stdlib-2.3.0 kotlinx-coroutines-core-jvm-1.8.0-intellij-14; do
		stockcp=$stockcp:$out/m2/$jar.jar
	done
	[ -d "$out/runksp-classes" ] && [ -d "$out/runksp-stock" ] || { echo "run.sh: run jvm/kc-aa/build.sh stock first" >&2; exit 1; }
	libs=$(paste -sd: "$work/libs.txt")
	procs=$(paste -sd: "$work/procs.txt")
	rm -rf "$work/kc" "$work/stock"
	LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
		-cp "$out/runksp-classes:$procs:$out/kc-aa.jar:$deps" RunKspKt "$src" "$work/kc" "$libs" > "$work/kc.txt"
	java -Dksp.languageVersion=2.3 -cp "$out/runksp-stock:$procs:$STOCK_PROCS_EXTRA$stockcp" RunKspKt "$src" "$work/stock" "$libs" > "$work/stock.txt"
	rm -rf "$work"/{kc,stock}/{caches,classes}
	diff -ru "$work/stock" "$work/kc"
	echo "room: kc generates what stock KSP does ($(find "$work/kc" -type f | wc -l) files)"
fi

exec "$root/bench/ksp/run.sh" "$src" "$work/libs.txt" "$work/procs.txt" "$@"
