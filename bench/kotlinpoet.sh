#!/bin/sh
# Builds square/kotlinpoet with Gradle twice, and runs its tests on kc's
# build of its JVM library, and writes bench/results/tests-kotlinpoet.md.
#
# 1. The control build, as is, with kotlinc.
# 2. A build with kc compiling the :kotlinpoet JVM library
#    (bench/kotlinpoet/kc.init.gradle.kts). kotlinc compiles kotlinpoet's
#    tests and the interop modules against kc's classes, through their
#    Kotlin metadata, and inlines kc's inline functions; the
#    binary-compatibility validator checks kc's API.
# 3. The control build's tests, compiled by kotlinc against kotlinc's
#    library, run with JUnit 4 and java -Xverify:all on kotlinc's classes
#    and then with kc's in their place. This needs no metadata from kc,
#    only binary compatibility.
#
#   bench/kotlinpoet.sh               # several minutes
#   TAG=2.3.0 bench/kotlinpoet.sh     # another release
#
# Needs: go, git, rsync, a JDK (JAVA_HOME), and for the first run network
# access to github.com, services.gradle.org, plugins.gradle.org and Maven
# Central, for Gradle and the tests' dependencies.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
: "${TMPDIR:=/tmp}"
tag=${TAG:-2.4.0}
src="$TMPDIR/kc-survey/kotlinpoet-$tag"
work="$TMPDIR/kc-kotlinpoet"
results="$root/bench/results"

if [ ! -d "$src" ]; then
	mkdir -p "$(dirname "$src")"
	git -c advice.detachedHead=false clone -q --depth 1 --branch "$tag" https://github.com/square/kotlinpoet.git "$src"
fi
rm -rf "$work"
mkdir -p "$work" "$results"
cd "$root"
go build -o "$work/bin/kc" ./cmd/kc

# Fresh copies, without the clone's build outputs.
for build in control kc; do
	rsync -a --exclude build --exclude .gradle --exclude .kotlin "$src/" "$work/$build/"
done
gradle_tasks=":kotlinpoet:jvmTest :kotlinpoet:jvmApiCheck :interop:kotlin-metadata:test :interop:ksp:compileKotlin :interop:ksp:test-processor:test"
run() {
	build=$1
	shift
	echo "building kotlinpoet with Gradle ($build)..."
	# shellcheck disable=SC2086 # the tasks are words
	(cd "$work/$build" && ./gradlew --no-daemon --no-build-cache --continue -Pkotlin.incremental=false "$@" $gradle_tasks) \
		>"$work/$build/build.log" 2>&1 || true
}
# The control build also prints where its tests are and their class path.
run control --init-script "$root/bench/kotlinpoet/classpath.init.gradle.kts" :kotlinpoet:printJvmTestClasspath
KC="$work/bin/kc" run kc --init-script "$root/bench/kotlinpoet/kc.init.gradle.kts"

# The control build's tests, on each build's library classes.
value() { sed -n "s/^$1=//p" "$work/control/build.log"; }
tests=$(value KC_TEST_CLASSES)
java_tests=$(value KC_JAVA_TEST_CLASSES)
resources=$(value KC_TEST_RESOURCES)
main=$(value KC_MAIN_CLASSES)
cp=$(value KC_RUNTIME_CLASSPATH)
[ -n "$cp" ] || { tail -30 "$work/control/build.log"; exit 1; }
# Gradle may print $work resolved, so only the build's directory is swapped.
kc_main=$(echo "$main" | sed "s|/control/kotlinpoet/build/|/kc/kotlinpoet/build/|")
[ -d "$kc_main" ] || { echo "no kc build of the library at $kc_main"; exit 1; }
kc_cp=$(echo "$cp" | sed "s|$main|$kc_main|")
[ "$kc_cp" != "$cp" ] || { echo "kotlinc's library classes ($main) are not on the test class path"; exit 1; }
javac -nowarn -d "$work/runner" -cp "$cp" "$root/bench/kotlinpoet/RunTests.java" 2>/dev/null
junit() {
	echo "running the tests on $1's classes..."
	java -Xverify:all -cp "$work/runner:$tests:$java_tests:$resources:$2" RunTests "$tests" "$work/$1.tsv" >"$work/$1.log" 2>&1
}
junit kotlinc "$cp"
junit kc "$kc_cp"

status=0
go run ./bench/kotlinpoet -control "$work/control" -kc "$work/kc" \
	-control-tsv "$work/kotlinc.tsv" -kc-tsv "$work/kc.tsv" \
	-header "square/kotlinpoet $tag; $(java -version 2>&1 | head -1)." \
	-o "$results/tests-kotlinpoet.md" || status=$?
echo "wrote bench/results/tests-kotlinpoet.md"
sed -n '/^| Tests/,/^$/p' "$results/tests-kotlinpoet.md"
exit $status
