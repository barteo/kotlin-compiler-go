#!/usr/bin/env bash
# build.sh builds kc's fork of KSP2's kotlin-analysis-api module: KSP at tag
# $KSP_TAG with the patches in jvm/ksp/patches applied, compiled against the
# unshaded Analysis API 2.4.21 jar that jvm/aa-api/build.sh makes.
#
#	jvm/ksp/build.sh [workdir]
#
# It writes $workdir/ksp-aa-kc.jar (default workdir: jvm/ksp/build). The
# patched KSP checkout is $workdir/ksp, a git repository whose branch kc holds
# one commit per patch, so changing the fork is: edit and commit there, then
#
#	git -C $workdir/ksp format-patch --zero-commit --no-signature \
#		-o jvm/ksp/patches $KSP_COMMIT
#
# The jar leaves out the parts of the module that need the Analysis API
# implementation (the standalone FIR session, whose artifacts live on
# JetBrains' Maven repository) and the Java stubs:
#   - impl/backend/fir/ (the stock "aa" KspBackend and FirExtras),
#   - standalone/,
#   - src/main/java.
# What remains is KSP's processing loop, its KS* symbol model and command
# line, which talk to the Analysis API through public API, KcExtras and
# KspBackend only.
set -euo pipefail

KSP_TAG=2.3.9
KSP_COMMIT=799f6c17b4a84acd9d9f15e53a6d0022f442ab95
VERSION=2.4.21
MAVEN=${MAVEN_REPO:-https://repo1.maven.org/maven2}

here=$(cd "$(dirname "$0")" && pwd)
work=${1:-$here/build}
mkdir -p "$work/m2"
work=$(cd "$work" && pwd)
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}

# The Analysis API jar and its dependencies, in $work/aa/m2.
"$here/../aa-api/build.sh" "$work/aa" >/dev/null
aa=$work/aa/kc-aa-api-$VERSION.jar
lib=$work/aa/m2

# fetch downloads a Maven artifact into $work/m2, retrying on the 429s
# Maven Central sends to busy clients.
fetch() {
	local path=$1 out=$work/m2/$(basename "$1")
	[ -s "$out" ] && return
	for i in 1 2 3 4 5 6 7 8; do
		curl -sSfL -o "$out.tmp" "$MAVEN/$path" && mv "$out.tmp" "$out" && return
		sleep $((i * 3))
	done
	echo "build.sh: can't download $MAVEN/$path" >&2
	exit 1
}
fetch com/google/devtools/ksp/symbol-processing-api/$KSP_TAG/symbol-processing-api-$KSP_TAG.jar
fetch com/google/devtools/ksp/symbol-processing-common-deps/$KSP_TAG/symbol-processing-common-deps-$KSP_TAG.jar
fetch org/jetbrains/kotlinx/kotlinx-serialization-core-jvm/1.6.3/kotlinx-serialization-core-jvm-1.6.3.jar
fetch org/jetbrains/kotlinx/kotlinx-serialization-json-jvm/1.6.3/kotlinx-serialization-json-jvm-1.6.3.jar

# A shallow checkout of $KSP_TAG with the patches on branch kc.
src=$work/ksp
patches=("$here"/patches/*.patch)
stamp=$(cat "${patches[@]}" | git hash-object --stdin)
if [ "$(cat "$src/.git/kc-patches" 2>/dev/null)" != "$stamp" ]; then
	rm -rf "$src"
	git init -q "$src"
	git -C "$src" remote add origin https://github.com/google/ksp.git
	git -C "$src" fetch -q --depth 1 origin $KSP_COMMIT
	git -C "$src" checkout -q -b kc FETCH_HEAD
	git -C "$src" -c user.name=kc -c user.email=kc@localhost am -q --committer-date-is-author-date "${patches[@]}"
	echo "$stamp" > "$src/.git/kc-patches"
fi

(cd "$src" && find kotlin-analysis-api/src/main/kotlin common-util/src/main -name '*.kt' \
	-not -path '*/backend/fir/*' -not -path '*/standalone/*' | sort) > "$work/sources.txt"
sed -i "s|^|$src/|" "$work/sources.txt"

runtime=$lib/kotlin-compiler-$VERSION.jar:$lib/kotlin-stdlib-$VERSION.jar:$lib/kotlin-reflect-$VERSION.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar
m2=$work/m2
classpath=$aa:$runtime:$lib/caffeine-2.9.3.jar
classpath=$classpath:$m2/symbol-processing-api-$KSP_TAG.jar:$m2/symbol-processing-common-deps-$KSP_TAG.jar
classpath=$classpath:$m2/kotlinx-serialization-core-jvm-1.6.3.jar:$m2/kotlinx-serialization-json-jvm-1.6.3.jar
optins=
for m in KaImplementationDetail KaExperimentalApi KaNonPublicApi KaIdeApi KaPlatformInterface KaContextParameterApi; do
	optins="$optins -opt-in=org.jetbrains.kotlin.analysis.api.$m"
done
out=$work/classes
rm -rf "$out"
java -Xmx4g -cp "$runtime" org.jetbrains.kotlin.cli.jvm.K2JVMCompiler \
	-no-stdlib -no-reflect -cp "$classpath" -d "$out" \
	-module-name symbol-processing-aa -jvm-target 17 -jvm-default=enable $optins \
	-nowarn @"$work/sources.txt"
# The service files; the stock backend they name is not in this jar.
cp -R "$src/kotlin-analysis-api/src/main/resources/." "$out/"
rm -f "$out/META-INF/services/com.google.devtools.ksp.impl.backend.KspBackend"
rmdir -p --ignore-fail-on-non-empty "$out/META-INF/services"
jar=$work/ksp-aa-kc.jar
rm -f "$jar"
(cd "$out" && jar cf "$jar" .)
echo "$jar"
