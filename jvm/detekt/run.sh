#!/usr/bin/env bash
# run.sh runs detekt on kc, as build.sh built it. Arguments are
# detekt-cli's.
#
#	jvm/detekt/run.sh -i src --analysis-mode full -cp ... --jdk-home ...
#
# It runs detekt with --parallel, a class-data sharing archive and the
# parallel collector.
# KC_JAVA_OPTS adds JVM options: -Dkc.trace=true prints kc's timings, the
# Analysis API members it doesn't answer and how often it answers the
# others, and -Dkc.strict=true makes the unanswered ones fail.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}
[ -s "$here/build/classpath.txt" ] || { echo "run.sh: run build.sh first" >&2; exit 1; }
cp=$(cat "$here/build/classpath.txt")
# A class-data sharing archive of the classes detekt on kc loads: the JVM
# writes it at the end of the first run and maps it on later ones, instead
# of reading, parsing and verifying the thousands of classes it loads from
# jars. JDK 19 and later. The JVM rejects an
# archive once a jar on the class path changes, as kc-aa.jar does with
# every build, but doesn't write it again, so the archive is named by the
# jars' paths, sizes and times, and older ones are deleted.
cds=
java_version=$(sed -n 's/^JAVA_VERSION="\([0-9]*\).*/\1/p' "$java_home/release" 2>/dev/null || true)
if [ "${java_version:-0}" -ge 19 ]; then
	key=$(echo "$cp" | tr ':' '\n' | { xargs stat -L -c '%n %s %Y' 2>/dev/null || true; } | cksum | cut -d' ' -f1)
	mkdir -p "$here/build/cds"
	archive=$here/build/cds/detekt-$key.jsa
	for old in "$here"/build/cds/detekt-*.jsa; do
		[ "$old" = "$archive" ] || rm -f "$old"
	done
	# A run that stops early, on a usage error, writes an archive of the
	# few classes it loaded, and the JVM keeps it: an archive of detekt's
	# run is some 40 MB.
	if [ -f "$archive" ] && [ "$(stat -c %s "$archive")" -lt 8000000 ]; then rm -f "$archive"; fi
	cds="-XX:SharedArchiveFile=$archive -XX:+AutoCreateSharedArchive"
fi
# The run is CPU-bound on all cores, and the parallel collector costs the
# rules' threads less than G1's barriers and concurrent work.
# KC_JAVA_OPTS may choose another.
gc=-XX:+UseParallelGC
case ${KC_JAVA_OPTS:-} in *GC*) gc= ;; esac
# detekt runs its rules on all cores: kc's session is thread-safe, and its
# findings are the same as a serial run's.
parallel=--parallel
for a in "$@"; do [ "$a" = --parallel ] && parallel=; done
# Go's runtime installs signal handlers in the JVM's process; libjsig
# chains them with the JVM's.
# shellcheck disable=SC2086 # the options are words
LD_PRELOAD=$java_home/lib/libjsig.so exec java -Xss8m $cds $gc ${KC_JAVA_OPTS:-} -Dkc.lib="$here/../kc-aa/build/libkc.so" \
	-Didea.home.path="$here/build" -cp "$cp" dev.detekt.cli.Main $parallel "$@"
