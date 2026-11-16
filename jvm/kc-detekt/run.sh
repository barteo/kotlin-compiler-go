#!/usr/bin/env bash
# run.sh runs detekt on kc: detekt-cli, unchanged, with kc-detekt.jar in
# place of detekt-kotlin-analysis-api-standalone, so that the Analysis API
# session detekt builds is kc's. Arguments are detekt-cli's.
#
#	DETEKT_HOME=.../detekt-cli jvm/kc-detekt/run.sh -i src --analysis-mode full -cp ...
#
# KC_JAVA_OPTS adds JVM options: -Dkc.trace=true prints kc's timings and the
# Analysis API members it doesn't answer, -Dkc.strict=true makes those fail.
# build.sh must have run.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
: "${DETEKT_HOME:?set DETEKT_HOME to a detekt-cli distribution}"
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}

# detekt-cli's class path, in its start script's order.
cp=$(sed -n 's/^CLASSPATH=//p' "$DETEKT_HOME/bin/detekt-cli" | tr ':' '\n' |
	grep -v 'detekt-kotlin-analysis-api-standalone' | sed "s|\$APP_HOME|$DETEKT_HOME|" | paste -sd: -)
# Go's runtime installs signal handlers in the JVM's process; libjsig
# chains them with the JVM's.
LD_PRELOAD=$java_home/lib/libjsig.so exec java ${KC_JAVA_OPTS:-} -Dkc.lib="$here/build/libkc.so" \
	-cp "$here/build/kc-detekt.jar:$cp" dev.detekt.cli.Main "$@"
