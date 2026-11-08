#!/usr/bin/env bash
# run.sh times Hilt's KSP processors (hilt-compiler 2.59.2, with
# dagger-compiler's) with bench/ksp/run.sh, on stock KSP2 and on the KSP
# fork on kc.
#
#	bench/ksp/hilt/run.sh [RUNS]
#
# WORKLOAD picks the sources: "small" is jvm/kc-aa/test/hilt/src (one
# file); "large" (the default) is gen.sh's module of N (default 100)
# files, written to data/large-N. Before timing, it runs the processors
# once on each backend (the build.sh test's RunKsp) and fails unless both
# generate the same files (CHECK=0 skips this). EDIT=auto edits
# h/Feature000.kt (large) or h/App.kt (small). The rest of the environment
# is bench/ksp/run.sh's (BACKENDS, COLD, JFR, KC_TIMING, JVM_OPTS). It
# needs jvm/kc-aa/build.sh stock to have run, which fetches Hilt's jars.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
. "$root/bench/ksp/workload.sh"
. "$kcaa/hilt-jars.sh"

work=$out/hilt-bench
mkdir -p "$work"
case ${WORKLOAD:-large} in
small)
	src=$kcaa/test/hilt/src
	edit=h/App.kt
	;;
large)
	n=${N:-100}
	src=$here/data/large-$n
	[ -s "$src/h/Feature000.kt" ] && [ "$src/h/Feature000.kt" -nt "$here/gen.sh" ] || "$here/gen.sh" "$src" "$n"
	edit=h/Feature000.kt
	;;
*) echo "run.sh: WORKLOAD is small or large" >&2; exit 2 ;;
esac
{ echo "$lib/kotlin-stdlib-2.4.21.jar"; paths "${hilt_libs[@]}"; } > "$work/libs.txt"
paths "${hilt_procs[@]}" > "$work/procs.txt"
[ "${EDIT:-}" = auto ] && export EDIT=$edit
[ "${CHECK:-1}" = 0 ] || same_output "$src" "$work/libs.txt" "$work/procs.txt" "$work"

exec "$root/bench/ksp/run.sh" "$src" "$work/libs.txt" "$work/procs.txt" "$@"
