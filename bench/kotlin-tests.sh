#!/bin/sh
# Runs the Kotlin compiler's own tests against kc (see bench/conformance
# and docs/conformance.md), from a sparse checkout of JetBrains/kotlin at
# the tag of the Kotlin that KOTLIN_HOME is. Writes bench/results/MODE.md
# and bench/results/MODE.tsv, one line per test.
#
#   bench/kotlin-tests.sh box               # the box tests: compile, run, box() == "OK"
#   bench/kotlin-tests.sh diagnostics       # the diagnostics tests: reject what kotlinc rejects
#   bench/kotlin-tests.sh box -filter when/ # only some; other flags go to the harness
#   bench/kotlin-tests.sh box -baseline bench/results/box.tsv -tsv "" -o ""
#   bench/kotlin-tests.sh box -kotlinc kotlinc -sample 100 -o "" -tsv ""   # check the harness
#
# Needs: go, git and network access to github.com for the first run, a
# JDK (JAVA_HOME) and Kotlin (KOTLIN_HOME), as for bench/run.sh.
set -eu
mode=${1:?usage: bench/kotlin-tests.sh box|diagnostics [harness flags]}
shift
root=$(cd "$(dirname "$0")/.." && pwd)
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
: "${TMPDIR:=/tmp}"
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
tag=v$(cat "$kotlin_home/build.txt")
repo="$TMPDIR/kc-kotlin-tests"

if [ "$(cat "$repo/.kc-tag" 2>/dev/null)" != "$tag" ]; then
	rm -rf "$repo"
	echo "cloning JetBrains/kotlin $tag (sparse)"
	git -c advice.detachedHead=false clone -q --depth 1 --filter=blob:none --sparse --branch "$tag" \
		https://github.com/JetBrains/kotlin.git "$repo"
	echo "$tag" >"$repo/.kc-tag"
fi
git -C "$repo" sparse-checkout set --no-cone \
	/compiler/testData/codegen/box/ \
	/compiler/testData/diagnostics/ \
	/compiler/fir/checkers/gen/ \
	/compiler/fir/checkers/checkers.jvm/gen/ \
	/compiler/fir/raw-fir/raw-fir.common/gen/org/jetbrains/kotlin/fir/builder/FirSyntaxErrors.kt \
	/compiler/ir/backend.jvm/src/org/jetbrains/kotlin/backend/jvm/JvmBackendErrors.kt \
	/compiler/ir/backend.common/src/org/jetbrains/kotlin/backend/common/CommonBackendErrors.kt

cd "$root"
go build -o bin/kc ./cmd/kc
go build -o "$TMPDIR/kc-conformance-harness" ./bench/conformance
"$TMPDIR/kc-conformance-harness" -mode "$mode" -kotlin "$repo" -tag "$tag" -kc bin/kc \
	-stdlib "$kotlin_home/lib/kotlin-stdlib.jar" -test "$kotlin_home/lib/kotlin-test.jar" -reflect "$kotlin_home/lib/kotlin-reflect.jar" \
	-java "$JAVA_HOME/bin/java" -javac "$JAVA_HOME/bin/javac" \
	-o "bench/results/$mode.md" -tsv "bench/results/$mode.tsv" "$@"
