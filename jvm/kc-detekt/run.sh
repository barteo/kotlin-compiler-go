#!/usr/bin/env bash
# run.sh runs detekt on kc: detekt-cli, unchanged, with kc-detekt.jar in
# place of detekt-kotlin-analysis-api-standalone, so that the Analysis API
# session detekt builds is kc's. Arguments are detekt-cli's.
#
#	DETEKT_HOME=.../detekt-cli jvm/kc-detekt/run.sh -i src --analysis-mode full -cp ...
#
# It runs detekt with --parallel and a class-data sharing archive, which
# KC_PARALLEL=0 and KC_CDS=0 turn off. KC_JAVA_OPTS adds JVM options:
# -Dkc.trace=true prints kc's timings and the Analysis API members it
# doesn't answer, -Dkc.strict=true makes those fail, -Dkc.eager=false and
# -Dkc.cache=false turn off the background analysis and the session's
# caches, and -Dkc.optIns=a,b adds -opt-in markers detekt-cli can't pass.
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
# It is kept by class path, which the JVM checks it against; JDK 19 and
# later. KC_CDS=0 turns it off.
cds=
java_version=$(sed -n 's/^JAVA_VERSION="\([0-9]*\).*/\1/p' "$java_home/release" 2>/dev/null || true)
if [ "${KC_CDS:-1}" != 0 ] && [ "${java_version:-0}" -ge 19 ]; then
	cds="-XX:SharedArchiveFile=$here/build/kc-detekt-$(printf %s "$cp" | cksum | cut -d' ' -f1).jsa -XX:+AutoCreateSharedArchive"
fi
# detekt runs its rules on all cores: kc's session is thread-safe, and its
# findings are the same as a serial run's. KC_PARALLEL=0 runs them serially.
parallel=--parallel
for a in "$@"; do [ "$a" = --parallel ] && parallel=; done
[ "${KC_PARALLEL:-1}" = 0 ] && parallel=
# Go's runtime installs signal handlers in the JVM's process; libjsig
# chains them with the JVM's.
# shellcheck disable=SC2086 # the options are words
LD_PRELOAD=$java_home/lib/libjsig.so exec java $cds ${KC_JAVA_OPTS:-} -Dkc.lib="$here/build/libkc.so" \
	-cp "$here/build/kc-detekt.jar:$cp" dev.detekt.cli.Main $parallel "$@"
