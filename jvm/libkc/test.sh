#!/usr/bin/env bash
# test.sh builds libkc.so and runs the JNI bridge's stress test (Stress.java)
# twice: plainly, and with the JDK's libjsig preloaded for signal chaining,
# which is how a Gradle daemon hosting the library should run.
#
#	jvm/libkc/test.sh [threads [callsPerThread]]
#	jvm/libkc/test.sh gradle [threads [callsPerThread]]
#
# With "gradle" it runs the test inside a Gradle daemon instead (gradle/),
# a fresh daemon for each of the two runs: three builds in it, each running
# the test as the KSP plugin runs KSP (gradle/build.gradle.kts). The first
# loads the library; the second reuses the class loader kept by class path;
# the third, which must fail, has another class path, so a new class
# loader, which can't load the library again (a JVM lets one class loader
# load a native library). GRADLE is the
# gradle command (default gradle).
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

if [ "${1:-}" = gradle ]; then
	shift
	g() { ${GRADLE:-gradle} -q --daemon -p "$here/gradle" --project-cache-dir "$out/gradle" stress \
		-Pclasses="$out/classes" -Plib="$out/libkc.so" -Pargs="$stress_args" "$@"; }
	daemon() {
		${GRADLE:-gradle} -q --stop -p "$here/gradle" >/dev/null
		echo "-- build 1"; g
		echo "-- build 2, same daemon"; g
		echo "-- build 3, a new class loader"
		mkdir -p "$out/extra"
		g -Pextra="$out/extra" > "$out/fresh.log" 2>&1 || true
		if grep -q 'already loaded in another classloader' "$out/fresh.log"; then
			echo "UnsatisfiedLinkError, as expected"
		else
			cat "$out/fresh.log"; echo "FAIL: a second class loader loaded the library" >&2; exit 1
		fi
		${GRADLE:-gradle} -q --stop -p "$here/gradle" >/dev/null
	}
	stress_args="$*"
	echo "== gradle daemon"
	daemon
	echo "== gradle daemon with libjsig"
	LD_PRELOAD=$java_home/lib/libjsig.so daemon
	exit 0
fi
stress() { java -Xss512k -Dkc.lib="$out/libkc.so" -cp "$out/classes" Stress "$@"; }
echo "== plain"
stress "$@"
echo "== with libjsig"
LD_PRELOAD=$java_home/lib/libjsig.so stress "$@"
