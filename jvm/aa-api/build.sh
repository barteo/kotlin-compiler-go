#!/usr/bin/env bash
# build.sh builds the Kotlin Analysis API's public API jar, version 2.4.21,
# from the Kotlin sources, because the published analysis-api artifacts live
# on JetBrains' Maven repository, which not every build host can reach.
#
#	jvm/aa-api/build.sh [workdir]
#
# It writes $workdir/kc-aa-api-2.4.21.jar (default workdir: jvm/aa-api/build)
# holding the analysis-api, analysis-internal-utils and
# analysis-api-platform-interface modules, plus the few plain value classes of
# analysis-api-impl-base that clients such as KSP construct directly
# ($implbase below), compiled against the unshaded
# org.jetbrains.kotlin:kotlin-compiler:2.4.21 from Maven Central, which also
# bundles IntelliJ core, Guava, ASM and kotlinx-collections-immutable.
set -euo pipefail

KOTLIN_TAG=v2.4.21
KOTLIN_COMMIT=9180ce6a7a7d3627f88b424717583f28ddca052d
VERSION=2.4.21
MAVEN=${MAVEN_REPO:-https://repo1.maven.org/maven2}

here=$(cd "$(dirname "$0")" && pwd)
work=${1:-$here/build}
mkdir -p "$work/m2"
work=$(cd "$work" && pwd)
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}

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
fetch org/jetbrains/kotlin/kotlin-compiler/$VERSION/kotlin-compiler-$VERSION.jar
fetch org/jetbrains/kotlin/kotlin-stdlib/$VERSION/kotlin-stdlib-$VERSION.jar
fetch org/jetbrains/kotlin/kotlin-reflect/$VERSION/kotlin-reflect-$VERSION.jar
fetch org/jetbrains/kotlinx/kotlinx-coroutines-core-jvm/1.8.0/kotlinx-coroutines-core-jvm-1.8.0.jar
fetch org/jetbrains/annotations/23.0.0/annotations-23.0.0.jar
# The versions gradle/libs.versions.toml pins at $KOTLIN_TAG.
fetch com/github/ben-manes/caffeine/caffeine/2.9.3/caffeine-2.9.3.jar
fetch org/checkerframework/checker-qual/3.12.0/checker-qual-3.12.0.jar
fetch io/opentelemetry/opentelemetry-api/1.41.0/opentelemetry-api-1.41.0.jar
fetch io/opentelemetry/opentelemetry-context/1.41.0/opentelemetry-context-1.41.0.jar

# A sparse, blobless, shallow checkout of the three modules' sources.
src=$work/kotlin
modules="analysis/analysis-api analysis/analysis-internal-utils analysis/analysis-api-platform-interface"
implbase_dir=analysis/analysis-api-impl-base/src/org/jetbrains/kotlin/analysis/api/impl/base
implbase="annotations/KaAnnotationImpl.kt annotations/KaBaseNamedAnnotationValue.kt types/KaBaseTypeProjection.kt"
if [ "$(git -C "$src" rev-parse HEAD 2>/dev/null)" != $KOTLIN_COMMIT ] || [ ! -d "$src/$implbase_dir" ]; then
	rm -rf "$src"
	git init -q "$src"
	git -C "$src" remote add origin https://github.com/JetBrains/kotlin.git
	git -C "$src" sparse-checkout set $modules $implbase_dir/annotations $implbase_dir/types
	git -C "$src" fetch -q --depth 1 --filter=blob:none origin $KOTLIN_COMMIT
	git -C "$src" checkout -q FETCH_HEAD
fi

# The sources, with one mechanical change: kotlin-compiler's copy of IntelliJ
# core has the bodies of the inline service()/serviceOrNull() helpers
# stripped, so calls to them can't be inlined; they become getService calls
# on the declared return type, which is what the helpers do.
patched=$work/src
rm -rf "$patched"
for m in $modules; do
	mkdir -p "$patched/$m"
	cp -R "$src/$m/src" "$patched/$m/"
done
for f in $implbase; do
	mkdir -p "$(dirname "$patched/$implbase_dir/$f")"
	cp "$src/$implbase_dir/$f" "$patched/$implbase_dir/$f"
done
find "$patched" -name '*.kt' -print0 | xargs -0 perl -0777 -pi -e '
	s/(\):\s*([\w.]+)(?:<[^=\n]*>)?\??\s*=[^\n]*?(?:\n\s*)?[\w.()]+?)\.(?:service|serviceOrNull)\(\)/$1.getService(${2}::class.java)/g;
	s/\.(?:service|serviceOrNull)<([\w.]+)>\(\)/.getService(${1}::class.java)/g;'
find "$patched" -name '*.kt' > "$work/sources.txt"

lib=$work/m2
runtime=$lib/kotlin-compiler-$VERSION.jar:$lib/kotlin-stdlib-$VERSION.jar:$lib/kotlin-reflect-$VERSION.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar
classpath=$runtime:$lib/caffeine-2.9.3.jar:$lib/checker-qual-3.12.0.jar:$lib/opentelemetry-api-1.41.0.jar:$lib/opentelemetry-context-1.41.0.jar
optins=
for m in KaImplementationDetail KaNonPublicApi KaIdeApi KaExperimentalApi KaPlatformInterface KaContextParameterApi KaSpiExtensionPoint; do
	optins="$optins -opt-in=org.jetbrains.kotlin.analysis.api.$m"
done
out=$work/classes
rm -rf "$out"
java -Xmx4g -cp "$runtime" org.jetbrains.kotlin.cli.jvm.K2JVMCompiler \
	-no-stdlib -no-reflect -cp "$classpath" -d "$out" \
	-module-name analysis-api -jvm-target 17 \
	-Xcontext-parameters -XXLanguage:+NameBasedDestructuring $optins \
	-nowarn @"$work/sources.txt"
# Service declarations and other resources of the platform interface.
for m in $modules; do
	[ -d "$src/$m/resources" ] && cp -R "$src/$m/resources/." "$out/"
done
jar=$work/kc-aa-api-$VERSION.jar
rm -f "$jar"
(cd "$out" && jar cf "$jar" .)
echo "$jar"
