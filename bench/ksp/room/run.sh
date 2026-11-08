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
. "$root/bench/ksp/workload.sh"
. "$kcaa/room-jars.sh"

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
# Stock KSP needs Room's kotlinx-serialization; the KSP fork brings its own.
STOCK_PROCS_EXTRA=$(paths "${room_serialization[@]}" | paste -sd:)
export STOCK_PROCS_EXTRA
[ "${EDIT:-}" = auto ] && export EDIT=$edit
[ "${CHECK:-1}" = 0 ] || same_output "$src" "$work/libs.txt" "$work/procs.txt" "$work" "$STOCK_PROCS_EXTRA"

exec "$root/bench/ksp/run.sh" "$src" "$work/libs.txt" "$work/procs.txt" "$@"
