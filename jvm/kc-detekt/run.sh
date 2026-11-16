#!/usr/bin/env bash
# run.sh runs detekt on kc: detekt-cli, unchanged, with kc-detekt.jar in
# place of detekt-kotlin-analysis-api-standalone, so that the Analysis API
# session detekt builds is kc's. Arguments are detekt-cli's.
#
#	DETEKT_HOME=.../detekt-cli jvm/kc-detekt/run.sh -i src --analysis-mode full -cp ...
#
# It runs detekt with --parallel and a class-data sharing archive.
# KC_JAVA_OPTS adds JVM options: -Dkc.trace=true prints kc's timings and
# the Analysis API members it doesn't answer, -Dkc.strict=true makes those
# fail, and -Dkc.optIns=a,b adds -opt-in markers detekt-cli can't pass.
# build.sh must have run.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
: "${DETEKT_HOME:?set DETEKT_HOME to a detekt-cli distribution}"
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}

# detekt-cli's class path, in its start script's order.
cp=$(sed -n 's/^CLASSPATH=//p' "$DETEKT_HOME/bin/detekt-cli" | tr ':' '\n' |
	grep -v 'detekt-kotlin-analysis-api-standalone' | sed "s|\$APP_HOME|$DETEKT_HOME|" | paste -sd: -)
# A class-data sharing archive of the classes detekt on kc loads: the JVM
# writes it at the end of the first run and maps it on later ones, instead
# of reading, parsing and verifying the 5,000 or so classes it loads from
# jars, a seventh of a run on a small module (docs/detekt-performance.md).
# JDK 19 and later. The JVM rejects an archive once a jar on the class
# path changes, as kc-detekt.jar does with every build, but doesn't write
# it again, so the archive is named by the jars' paths, sizes and times,
# and older ones are deleted.
cds=
java_version=$(sed -n 's/^JAVA_VERSION="\([0-9]*\).*/\1/p' "$java_home/release" 2>/dev/null || true)
if [ "${java_version:-0}" -ge 19 ]; then
	key=$(echo "$here/build/kc-detekt.jar:$cp" | tr ':' '\n' | { xargs stat -c '%n %s %Y' 2>/dev/null || true; } | cksum | cut -d' ' -f1)
	archive=$here/build/kc-detekt-$key.jsa
	for old in "$here"/build/kc-detekt-*.jsa; do
		[ "$old" = "$archive" ] || rm -f "$old"
	done
	cds="-XX:SharedArchiveFile=$archive -XX:+AutoCreateSharedArchive"
fi
# detekt runs its rules on all cores: kc's session is thread-safe, and its
# findings are the same as a serial run's.
parallel=--parallel
for a in "$@"; do [ "$a" = --parallel ] && parallel=; done
# Go's runtime installs signal handlers in the JVM's process; libjsig
# chains them with the JVM's.
# shellcheck disable=SC2086 # the options are words
LD_PRELOAD=$java_home/lib/libjsig.so exec java $cds ${KC_JAVA_OPTS:-} -Dkc.lib="$here/build/libkc.so" \
	-cp "$here/build/kc-detekt.jar:$cp" dev.detekt.cli.Main $parallel "$@"
