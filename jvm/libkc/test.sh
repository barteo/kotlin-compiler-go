#!/usr/bin/env bash
# test.sh builds libkc.so and runs the JNI bridge's stress test (Stress.java)
# twice: plainly, and with the JDK's libjsig preloaded for signal chaining,
# which is how a Gradle daemon hosting the library should run.
#
#	jvm/libkc/test.sh [threads [callsPerThread]]
#	jvm/libkc/test.sh gradle [threads [callsPerThread]]
#
# With "gradle" it runs the test inside a Gradle daemon instead (gradle/),
# a fresh daemon for each of the two runs: four builds in it, each running
# the test as the KSP plugin runs KSP (gradle/build.gradle.kts). The first
# loads the library, in KcBridge's shared class loader; the second reuses
# the class loader kept by class path; the third has another class path, so
# a new class loader, as the KSP plugin makes for another KSP class path,
# and must reach the library through
# the same shared class loader; the fourth has classes built for another
# ABI and must fail with KcBridge's message. GRADLE is the gradle command
# (default gradle).
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
	# The classes of a kc-aa built for another ABI, for the fourth build.
	rm -rf "$out/abi-classes"
	mkdir -p "$out/abi-src"
	sed 's|ABI_VERSION = "kc-aa/[^"]*"|ABI_VERSION = "kc-aa/test-other"|' "$here/src/io/github/barteo/kc/KcNative.java" \
		> "$out/abi-src/KcNative.java"
	javac -d "$out/abi-classes" "$out/abi-src/KcNative.java" \
		$(find "$here/src" "$here/test" -name '*.java' ! -name KcNative.java)
	g() { ${GRADLE:-gradle} -q --daemon -p "$here/gradle" --project-cache-dir "$out/gradle" stress \
		-Plib="$out/libkc.so" -Pargs="$stress_args" "$@"; }
	daemon() {
		${GRADLE:-gradle} -q --stop -p "$here/gradle" >/dev/null
		echo "-- build 1"; g -Pclasses="$out/classes" | tee "$out/build1.log"
		echo "-- build 2, same daemon"; g -Pclasses="$out/classes" | tee "$out/build2.log"
		echo "-- build 3, a new class loader"
		mkdir -p "$out/extra"
		g -Pclasses="$out/classes" -Pextra="$out/extra" | tee "$out/build3.log"
		# All three reach libkc through one class loader, KcBridge's.
		loaders=$(grep -h '^libkc: ' "$out/build1.log" "$out/build2.log" "$out/build3.log" | sort -u)
		if [ "$(echo "$loaders" | wc -l)" != 1 ] || ! grep -q '^libkc: class loader libkc@' <<< "$loaders"; then
			echo "FAIL: the builds didn't share KcBridge's class loader:" >&2; echo "$loaders" >&2; exit 1
		fi
		echo "one shared class loader: $loaders"
		echo "-- build 4, a kc-aa of another ABI"
		g -Pclasses="$out/abi-classes" --stacktrace > "$out/abi.log" 2>&1 || true
		if grep -q 'libkc has ABI kc-aa/[^ ]*, but this kc-aa expects kc-aa/test-other' "$out/abi.log"; then
			echo "UnsatisfiedLinkError on the ABI, as expected"
		else
			cat "$out/abi.log"; echo "FAIL: no ABI error" >&2; exit 1
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
