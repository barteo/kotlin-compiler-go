#!/usr/bin/env bash
# run.sh times Moshi's KSP codegen (moshi-kotlin-codegen 1.15.2) with
# bench/ksp/run.sh, on stock KSP2 and on the KSP fork on kc.
#
#	bench/ksp/moshi/run.sh [RUNS]
#
# WORKLOAD picks the sources: "small" is jvm/kc-aa/test/moshi/src (one
# file, four models); "large" (the default) is gen.sh's module of N
# (default 100) files, written to data/large-N. Before timing, it runs the
# codegen once on each backend (the build.sh test's RunKsp) and fails
# unless both generate the same files (CHECK=0 skips this). EDIT=auto edits
# m/Model000.kt (large) or m/Models.kt (small). The rest of the environment
# is bench/ksp/run.sh's (BACKENDS, COLD, JFR, KC_TIMING, JVM_OPTS). It
# needs jvm/kc-aa/build.sh stock to have run, which fetches Moshi's jars.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
. "$root/bench/ksp/workload.sh"

# build.sh's class paths for the Moshi test.
moshi_libs=(com.squareup.moshi:moshi:1.15.2 com.squareup.okio:okio-jvm:3.7.0)
moshi_procs=(com.squareup.moshi:moshi-kotlin-codegen:1.15.2 com.squareup.moshi:moshi:1.15.2
	com.squareup.okio:okio-jvm:3.7.0 com.squareup:kotlinpoet:1.13.2 com.squareup:kotlinpoet-ksp:1.13.2
	com.google.guava:guava:30.1.1-jre org.ow2.asm:asm:9.3)

work=$out/moshi-bench
mkdir -p "$work"
case ${WORKLOAD:-large} in
small)
	src=$kcaa/test/moshi/src
	edit=m/Models.kt
	;;
large)
	n=${N:-100}
	src=$here/data/large-$n
	[ -s "$src/m/Model000.kt" ] && [ "$src/m/Model000.kt" -nt "$here/gen.sh" ] || "$here/gen.sh" "$src" "$n"
	edit=m/Model000.kt
	;;
*) echo "run.sh: WORKLOAD is small or large" >&2; exit 2 ;;
esac
{ echo "$lib/kotlin-stdlib-2.4.21.jar"; paths "${moshi_libs[@]}"; } > "$work/libs.txt"
paths "${moshi_procs[@]}" > "$work/procs.txt"
[ "${EDIT:-}" = auto ] && export EDIT=$edit
[ "${CHECK:-1}" = 0 ] || same_output "$src" "$work/libs.txt" "$work/procs.txt" "$work"

exec "$root/bench/ksp/run.sh" "$src" "$work/libs.txt" "$work/procs.txt" "$@"
