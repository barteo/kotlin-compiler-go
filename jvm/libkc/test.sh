#!/usr/bin/env bash
# test.sh builds libkc.so and runs the JNI bridge's stress test (Stress.java)
# twice: plainly, and with the JDK's libjsig preloaded for signal chaining,
# which is how a Gradle daemon hosting the library should run.
#
#	jvm/libkc/test.sh [threads [callsPerThread]]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=$here/build
mkdir -p "$out/classes"
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}

(cd "$root" && CGO_CFLAGS="-I$java_home/include -I$java_home/include/linux" \
	go build -tags libkc -buildmode=c-shared -o "$out/libkc.so" ./cmd/libkc)
javac -d "$out/classes" $(find "$here/src" "$here/test" -name '*.java')

stress() { java -Xss512k -Dkc.lib="$out/libkc.so" -cp "$out/classes" Stress "$@"; }
echo "== plain"
stress "$@"
echo "== with libjsig"
LD_PRELOAD=$java_home/lib/libjsig.so stress "$@"
