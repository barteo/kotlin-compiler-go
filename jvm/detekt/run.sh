#!/usr/bin/env bash
# run.sh runs detekt on kc, as build.sh built it. Arguments are
# detekt-cli's.
#
#	jvm/detekt/run.sh -i src --analysis-mode full -cp ... --jdk-home ...
#
# KC_JAVA_OPTS adds JVM options: -Dkc.trace=true prints kc's timings and the
# Analysis API members it doesn't answer, -Dkc.strict=true makes those fail.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}
[ -s "$here/build/classpath.txt" ] || { echo "run.sh: run build.sh first" >&2; exit 1; }
# Go's runtime installs signal handlers in the JVM's process; libjsig
# chains them with the JVM's.
LD_PRELOAD=$java_home/lib/libjsig.so exec java -Xss8m ${KC_JAVA_OPTS:-} -Dkc.lib="$here/../kc-aa/build/libkc.so" \
	-Didea.home.path="$here/build" -cp "$(cat "$here/build/classpath.txt")" dev.detekt.cli.Main "$@"
