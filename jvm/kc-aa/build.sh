#!/usr/bin/env bash
# build.sh builds the kc KSP backend: the Analysis API answered by kc
# through libkc (jvm/kc-aa/src), and the libkc.so it loads.
#
#	jvm/kc-aa/build.sh [test|stock|ksp]
#
# It writes build/kc-aa.jar and build/libkc.so. With "test" it then runs
# KSP's processing loop on the kc backend over test/slice, with a processor
# that prints the annotated symbols (test/PrintAnnotated.kt), and compares
# the output with test/slice/want.txt; then it runs Moshi's KSP codegen over
# test/moshi/src (test/RunKsp.kt) and compares what it generates with
# test/moshi/want, and Dagger's, Room's and Hilt's the same way, over
# test/{dagger,room,hilt}/src. "stock" does what "test" does, and then runs
# each on stock KSP 2.3.6 (its embeddable Analysis API, Kotlin 2.3) over
# the same sources: the processors' output must match the same want
# directories, and the slice's want.txt but for test/slice/stock.diff.
# Room's jars come from Google's Maven repository (GOOGLE_MAVEN_REPO), the
# rest from Maven Central (MAVEN_REPO).
#
# "ksp" runs KSP's own unit tests (its kotlin-analysis-api test suite, with
# the processors of its test-utils) on the kc backend with ksptest/KspSuite.kt,
# writes build/ksptest/results.txt, and fails if a test test/ksp/pass.txt
# lists doesn't pass; KSP_UPDATE=1 rewrites pass.txt. KSP_TESTS=REGEX runs
# only the tests whose names match. test/ksp/skip.txt says why each other
# test fails. KSP_JAVA_OPTS adds JVM options (-Dkc.diagnostics prints kc's).
#
# The symbol classes extend generated stubs (build/Stubs.kt, made by
# gen/GenStubs.kt) whose members throw UnsupportedOperationException("kc:
# Class.member"), so a member kc doesn't answer yet fails with its name.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=$here/build
mkdir -p "$out"
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}

# The KSP fork, the Analysis API jar and their dependencies.
kspwork=$here/../ksp/build
if [ ! -s "$kspwork/ksp-aa-kc.jar" ] || [ -n "$(find "$here/../ksp/patches" -newer "$kspwork/ksp-aa-kc.jar")" ]; then
	"$here/../ksp/build.sh" "$kspwork" >/dev/null
fi
lib=$kspwork/aa/m2
m2=$kspwork/m2
aa=$kspwork/aa/kc-aa-api-2.4.21.jar
runtime=$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar
deps=$aa:$runtime:$lib/caffeine-2.9.3.jar:$kspwork/ksp-aa-kc.jar
deps=$deps:$m2/symbol-processing-api-2.3.9.jar:$m2/symbol-processing-common-deps-2.3.9.jar
deps=$deps:$m2/kotlinx-serialization-core-jvm-1.6.3.jar:$m2/kotlinx-serialization-json-jvm-1.6.3.jar
kotlinc() { java -Xmx3g -cp "$runtime" org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -no-stdlib -no-reflect "$@"; }

optins="-opt-in=org.jetbrains.kotlin.config.CompilerConfiguration.Internals -opt-in=org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi -opt-in=org.jetbrains.kotlin.CoreEnvironmentDeprecation"
for m in KaImplementationDetail KaExperimentalApi KaNonPublicApi KaIdeApi KaPlatformInterface KaContextParameterApi KaSpiExtensionPoint; do
	optins="$optins -opt-in=org.jetbrains.kotlin.analysis.api.$m"
done

# libkc.so.
(cd "$root" && CGO_CFLAGS="-I$java_home/include -I$java_home/include/linux" \
	go build -tags libkc -buildmode=c-shared -o "$out/libkc.so" ./cmd/libkc)

# The stubs of the Analysis API classes kc implements.
stubbed=(
	symbols.KaFileSymbol symbols.KaNamedClassSymbol symbols.KaNamedFunctionSymbol
	symbols.KaKotlinPropertySymbol symbols.KaConstructorSymbol symbols.KaValueParameterSymbol
	symbols.KaTypeParameterSymbol symbols.KaEnumEntrySymbol symbols.KaTypeAliasSymbol
	symbols.KaPropertyGetterSymbol symbols.KaPropertySetterSymbol symbols.KaReceiverParameterSymbol
	symbols.KaJavaFieldSymbol symbols.KaPackageSymbol symbols.KaBackingFieldSymbol
	symbols.KaDestructuringDeclarationSymbol symbols.KaSyntheticJavaPropertySymbol
	types.KaUsualClassType types.KaClassErrorType types.KaFunctionType types.KaTypeParameterType
	types.KaFlexibleType types.KaDefinitelyNotNullType types.KaErrorType
	annotations.KaAnnotation projectStructure.KaSourceModule projectStructure.KaLibraryModule
	symbols.pointers.KaSymbolPointer
)
rm -rf "$out/gen"
kotlinc -cp "$runtime" -d "$out/gen" "$here/gen/GenStubs.kt" 2>&1 | grep -v '^warning' || true
java -cp "$out/gen:$runtime:$aa:$lib/caffeine-2.9.3.jar" GenStubsKt "$out/Stubs.kt" \
	$(printf 'org.jetbrains.kotlin.analysis.api.%s ' "${stubbed[@]}")

# The backend.
classes=$out/classes
rm -rf "$classes"
mkdir -p "$classes"
javac -d "$classes" $(find "$here/../libkc/src" -name '*.java')
kotlinc -cp "$deps:$classes" -d "$classes" -module-name kc-aa -jvm-target 17 -jvm-default=enable $optins \
	-nowarn "$out/Stubs.kt" $(find "$here/src" -name '*.kt')
cp -R "$here/src/META-INF" "$classes/"
rm -f "$out/kc-aa.jar"
(cd "$classes" && jar cf "$out/kc-aa.jar" .)
echo "$out/kc-aa.jar"

mode=${1:-}
if [ "$mode" = ksp ]; then
	ksp=$kspwork/ksp
	kt=$out/ksptest
	rm -rf "$kt/classes"
	mkdir -p "$kt/classes"
	# The test processors: test-utils' and kotlin-analysis-api's own.
	kotlinc -cp "$deps" -d "$kt/classes" -jvm-target 17 -jvm-default=enable $optins -nowarn \
		$(find "$ksp/test-utils/src/main/kotlin" "$ksp/kotlin-analysis-api/src/test/kotlin/com/google/devtools/ksp/processor" -name '*.kt') \
		"$here/ksptest/KspSuite.kt"
	LD_PRELOAD=$java_home/lib/libjsig.so java -Xss8m ${KSP_JAVA_OPTS:-} -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
		-Dksptest.stdlib="$lib/kotlin-stdlib-2.4.21.jar" -Dksptest.annotations="$lib/annotations-23.0.0.jar" \
		-cp "$kt/classes:$out/kc-aa.jar:$deps" KspSuiteKt "$ksp" "$kt/work" "$kt/results.txt" \
		"$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar" \
		${KSP_TESTS:-} 2> "$kt/suite.log"
	[ -n "${KSP_TESTS:-}" ] && exit 0
	awk '$1 == "PASS" { print $2 }' "$kt/results.txt" | sort > "$kt/pass.txt"
	if [ "${KSP_UPDATE:-}" = 1 ]; then
		cp "$kt/pass.txt" "$here/test/ksp/pass.txt"
	fi
	lost=$(comm -23 "$here/test/ksp/pass.txt" "$kt/pass.txt")
	new=$(comm -13 "$here/test/ksp/pass.txt" "$kt/pass.txt")
	[ -z "$new" ] || echo "ksp: newly passing (KSP_UPDATE=1 records them):" $new
	if [ -n "$lost" ]; then
		echo "ksp: no longer passing:" $lost >&2
		exit 1
	fi
	# Every test that doesn't pass has a reason in skip.txt.
	missing=$(awk '$1 != "PASS" { print $2 }' "$kt/results.txt" | sort | comm -23 - <(cut -d: -f1 "$here/test/ksp/skip.txt" | sort))
	[ -z "$missing" ] || echo "ksp: failing without a reason in test/ksp/skip.txt:" $missing
	echo "ksp: ok ($(wc -l < "$kt/pass.txt") pass)"
	exit 0
fi
[ "$mode" = test ] || [ "$mode" = stock ] || exit 0

# The symbols PrintAnnotated prints (see its doc comment).
slice=(a.Marker a.Other kotlin.Suppress e.Args e.Inner kotlin.Deprecated java.beans.JavaBean
	decls:kotlin.ReplaceWith decls:kotlin.coroutines.AbstractCoroutineContextKey decls:java.util.AbstractMap.SimpleEntry decls:java.lang.Thread.State decls:b.E decls:kotlin.DeprecationLevel
	decls:f.Strings members:f.Strings members:f.Base members:f.Bag:get,size,toString assign:f.Assign sealed:f.Shape
	overrides:f.Strings#get:f.Named#get overrides:f.Strings#get:f.Source#get overrides:f.Strings#name:f.Source#get
	package:g package:kotlin.collections:listOf,lastIndex,ArrayList,List,emptyList,AbstractList package:java.util.function:Supplier
	fun:kotlin.collections.listOf fun:g.top prop:kotlin.collections.lastIndex prop:kotlin.coroutines.intrinsics.COROUTINE_SUSPENDED prop:g.topProp alias:g.Holder jvm:h.Sigs annots:a.Targets a.OnField a.FieldOrFun a.Anywhere)
rm -rf "$out/test-classes" "$out/slice"
kotlinc -cp "$deps:$out/kc-aa.jar" -d "$out/test-classes" -jvm-target 17 -nowarn "$here/test/PrintAnnotated.kt"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" \
	-Dkc.stdlib="$lib/kotlin-stdlib-2.4.21.jar" -Didea.home.path="$out" \
	-cp "$out/test-classes:$out/kc-aa.jar:$deps" PrintAnnotatedKt "$here/test/slice/src" "$out/slice" "${slice[@]}" \
	> "$out/slice.txt"
diff -u "$here/test/slice/want.txt" "$out/slice.txt"
echo "slice: ok"

# Moshi's codegen, a real processor, from Maven Central.
MAVEN=${MAVEN_REPO:-https://repo1.maven.org/maven2}
GMAVEN=${GOOGLE_MAVEN_REPO:-https://dl.google.com/dl/android/maven2}
mkdir -p "$out/m2"
# fetch PATH [REPO] downloads PATH from REPO (Maven Central by default)
# into build/m2.
fetch() {
	local path=$1 repo=${2:-$MAVEN} dst=$out/m2/$(basename "$1")
	[ -s "$dst" ] && return
	for i in 1 2 3 4 5 6 7 8; do
		curl -sSfL -o "$dst.tmp" "$repo/$path" && mv "$dst.tmp" "$dst" && return
		sleep $((i * 3))
	done
	echo "build.sh: can't download $repo/$path" >&2
	exit 1
}
# jars GROUP:ARTIFACT:VERSION... fetches those jars, androidx's from
# Google's Maven repository, and prints their class path, ":"-prefixed.
jars() {
	local c g a v repo cp=
	for c; do
		IFS=: read -r g a v <<< "$c"
		repo=$MAVEN
		[[ $g == androidx.* ]] && repo=$GMAVEN
		fetch "${g//.//}/$a/$v/$a-$v.jar" "$repo"
		cp=$cp:$out/m2/$a-$v.jar
	done
	echo "$cp"
}

# Stock KSP and RunKsp built against it.
if [ "$mode" = stock ]; then
	stockcp=
	for jar in com/google/devtools/ksp/symbol-processing-aa-embeddable/2.3.6/symbol-processing-aa-embeddable-2.3.6.jar \
		com/google/devtools/ksp/symbol-processing-api/2.3.6/symbol-processing-api-2.3.6.jar \
		com/google/devtools/ksp/symbol-processing-common-deps/2.3.6/symbol-processing-common-deps-2.3.6.jar \
		org/jetbrains/kotlin/kotlin-stdlib/2.3.0/kotlin-stdlib-2.3.0.jar \
		org/jetbrains/intellij/deps/kotlinx/kotlinx-coroutines-core-jvm/1.8.0-intellij-14/kotlinx-coroutines-core-jvm-1.8.0-intellij-14.jar; do
		fetch "$jar"
		stockcp=$stockcp:$out/m2/$(basename "$jar")
	done
	rm -rf "$out/runksp-stock"
	kotlinc -cp "${stockcp#:}" -language-version 2.3 -api-version 2.3 -d "$out/runksp-stock" -jvm-target 17 -nowarn \
		"$here/test/RunKsp.kt"
	# The slice on stock KSP differs from want.txt only as recorded in
	# test/slice/stock.diff: what kc doesn't model yet (see
	# docs/analysis-api.md, step 10).
	rm -rf "$out/test-classes-stock" "$out/stock-slice"
	kotlinc -cp "${stockcp#:}" -language-version 2.3 -api-version 2.3 -d "$out/test-classes-stock" -jvm-target 17 -nowarn \
		"$here/test/PrintAnnotated.kt"
	java -Dksp.languageVersion=2.3 -Dkc.stdlib="$lib/kotlin-stdlib-2.4.21.jar" -cp "$out/test-classes-stock$stockcp" \
		PrintAnnotatedKt "$here/test/slice/src" "$out/stock-slice" "${slice[@]}" > "$out/stock-slice.txt"
	{ diff "$here/test/slice/want.txt" "$out/stock-slice.txt" || true; } | grep '^[<>]' > "$out/stock-slice.diff" || true
	diff -u "$here/test/slice/stock.diff" "$out/stock-slice.diff"
	echo "slice: stock KSP differs as recorded"
fi
# stock NAME SRC LIBS PROCS runs PROCS on stock KSP over SRC, with
# libraries LIBS, and compares the output with test/NAME/want.
stock() {
	[ "$mode" = stock ] || return 0
	rm -rf "$out/stock-$1"
	java -Dksp.languageVersion=2.3 -cp "$out/runksp-stock$4$stockcp" RunKspKt "$2" "$out/stock-$1" "$3" > "$out/stock-$1.txt"
	rm -rf "$out/stock-$1/caches" "$out/stock-$1/classes"
	diff -ru "$here/test/$1/want" "$out/stock-$1"
	echo "$1: stock KSP ok"
}

procs=
for jar in com/squareup/moshi/moshi-kotlin-codegen/1.15.2/moshi-kotlin-codegen-1.15.2.jar \
	com/squareup/moshi/moshi/1.15.2/moshi-1.15.2.jar \
	com/squareup/okio/okio-jvm/3.7.0/okio-jvm-3.7.0.jar \
	com/squareup/kotlinpoet/1.13.2/kotlinpoet-1.13.2.jar \
	com/squareup/kotlinpoet-ksp/1.13.2/kotlinpoet-ksp-1.13.2.jar \
	com/google/guava/guava/30.1.1-jre/guava-30.1.1-jre.jar \
	org/ow2/asm/asm/9.3/asm-9.3.jar; do
	fetch "$jar"
	procs=$procs:$out/m2/$(basename "$jar")
done

rm -rf "$out/runksp-classes" "$out/moshi"
kotlinc -cp "$deps:$out/kc-aa.jar" -d "$out/runksp-classes" -jvm-target 17 -nowarn "$here/test/RunKsp.kt"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes:$out/kc-aa.jar:$deps$procs" RunKspKt \
	"$here/test/moshi/src" "$out/moshi" "$lib/kotlin-stdlib-2.4.21.jar:$out/m2/moshi-1.15.2.jar:$out/m2/okio-jvm-3.7.0.jar" \
	> "$out/moshi.txt"
rm -rf "$out/moshi/caches"
diff -ru "$here/test/moshi/want" "$out/moshi"
echo "moshi: ok"
# Asked for incremental processing, KSP rebuilds on every run on kc, which
# records no lookups: the same output, and the caches' up-to-date marker,
# here left from an earlier run, deleted so that the next run rebuilds too.
rm -rf "$out/moshi-ic"
mkdir -p "$out/moshi-ic/caches"
touch "$out/moshi-ic/caches/caches.uptodate"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dksp.incremental=true -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes:$out/kc-aa.jar:$deps$procs" RunKspKt \
	"$here/test/moshi/src" "$out/moshi-ic" "$lib/kotlin-stdlib-2.4.21.jar:$out/m2/moshi-1.15.2.jar:$out/m2/okio-jvm-3.7.0.jar" \
	> "$out/moshi-ic.txt"
if [ -e "$out/moshi-ic/caches/caches.uptodate" ]; then
	echo "incremental: caches.uptodate kept on kc" >&2
	exit 1
fi
rm -rf "$out/moshi-ic/caches"
diff -ru "$here/test/moshi/want" "$out/moshi-ic"
echo "incremental: rebuilds on kc (ok)"
stock moshi "$here/test/moshi/src" "$lib/kotlin-stdlib-2.4.21.jar:$out/m2/moshi-1.15.2.jar:$out/m2/okio-jvm-3.7.0.jar" "$procs"

# Dagger's KSP processor (XProcessing) over test/dagger/src. Its jars come
# first: kotlin-compiler bundles an older Guava, and KSP proper would load
# processors in a class loader of their own.
procs=
for jar in com/google/dagger/dagger-compiler/2.59.2/dagger-compiler-2.59.2.jar \
	com/google/dagger/dagger-spi/2.59.2/dagger-spi-2.59.2.jar \
	com/google/dagger/dagger/2.59.2/dagger-2.59.2.jar \
	javax/inject/javax.inject/1/javax.inject-1.jar \
	jakarta/inject/jakarta.inject-api/2.0.1/jakarta.inject-api-2.0.1.jar \
	com/google/code/findbugs/jsr305/3.0.2/jsr305-3.0.2.jar \
	com/google/guava/guava/33.0.0-jre/guava-33.0.0-jre.jar \
	com/google/guava/failureaccess/1.0.2/failureaccess-1.0.2.jar \
	com/squareup/javapoet/1.13.0/javapoet-1.13.0.jar \
	com/squareup/kotlinpoet/1.11.0/kotlinpoet-1.11.0.jar \
	com/google/googlejavaformat/google-java-format/1.33.0/google-java-format-1.33.0.jar \
	org/jetbrains/kotlin/kotlin-metadata-jvm/2.2.20/kotlin-metadata-jvm-2.2.20.jar; do
	fetch "$jar"
	procs=$procs:$out/m2/$(basename "$jar")
done
rm -rf "$out/dagger"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes$procs:$out/kc-aa.jar:$deps" RunKspKt \
	"$here/test/dagger/src" "$out/dagger" \
	"$lib/kotlin-stdlib-2.4.21.jar:$out/m2/dagger-2.59.2.jar:$out/m2/javax.inject-1.jar:$out/m2/jakarta.inject-api-2.0.1.jar" \
	> "$out/dagger.txt"
rm -rf "$out/dagger/caches"
diff -ru "$here/test/dagger/want" "$out/dagger"
echo "dagger: ok"
stock dagger "$here/test/dagger/src" \
	"$lib/kotlin-stdlib-2.4.21.jar:$out/m2/dagger-2.59.2.jar:$out/m2/javax.inject-1.jar:$out/m2/jakarta.inject-api-2.0.1.jar" "$procs"

# Room's processor (XProcessing, KotlinPoet output) over test/room/src,
# and Hilt's with Dagger's over test/hilt/src: the class paths Gradle
# resolves for room-compiler 2.8.4 and hilt-compiler 2.59.2, but for KSP's
# API and the Kotlin standard library, which KSP provides. On kc, Room's
# kotlinx-serialization is the one KSP's fork already has.
roomlibs=$lib/kotlin-stdlib-2.4.21.jar$(jars androidx.room:room-runtime-jvm:2.8.4 androidx.room:room-common-jvm:2.8.4 \
	androidx.collection:collection-jvm:1.5.0 androidx.sqlite:sqlite-jvm:2.6.2 androidx.annotation:annotation-jvm:1.9.1 \
	org.jetbrains.kotlinx:kotlinx-coroutines-core-jvm:1.8.1 org.jetbrains:annotations:23.0.0)
procs=$(jars androidx.room:room-migration-jvm:2.8.4 androidx.room:room-common-jvm:2.8.4 androidx.room:room-external-antlr:2.8.4 \
	androidx.room:room-compiler-processing:2.8.4 androidx.room:room-compiler:2.8.4 com.google.auto:auto-common:1.2.1 \
	com.google.auto.value:auto-value-annotations:1.6.3 com.intellij:annotations:12.0 com.squareup:kotlinpoet-javapoet:2.0.0 \
	com.squareup:javapoet:1.13.0 commons-codec:commons-codec:1.15 org.jetbrains.kotlin:kotlin-metadata-jvm:2.2.0 \
	com.squareup:kotlinpoet-jvm:2.0.0 org.jetbrains.kotlin:kotlin-reflect:2.0.10 androidx.annotation:annotation-jvm:1.9.1 \
	org.xerial:sqlite-jdbc:3.41.2.2 com.google.guava:guava:33.2.1-jre org.jspecify:jspecify:1.0.0 org.jetbrains:annotations:13.0 \
	com.google.guava:failureaccess:1.0.2 com.google.code.findbugs:jsr305:3.0.2 org.checkerframework:checker-qual:3.42.0 \
	com.google.errorprone:error_prone_annotations:2.26.1)
serialization=$(jars org.jetbrains.kotlinx:kotlinx-serialization-core-jvm:1.8.1 org.jetbrains.kotlinx:kotlinx-serialization-json-jvm:1.8.1)
rm -rf "$out/room"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes$procs:$out/kc-aa.jar:$deps" RunKspKt "$here/test/room/src" "$out/room" "$roomlibs" \
	> "$out/room.txt"
rm -rf "$out/room/caches" "$out/room/classes"
diff -ru "$here/test/room/want" "$out/room"
echo "room: ok"
stock room "$here/test/room/src" "$roomlibs" "$procs$serialization"

hiltlibs=$lib/kotlin-stdlib-2.4.21.jar$(jars com.google.dagger:hilt-core:2.59.2 com.google.dagger:dagger:2.59.2 \
	com.google.code.findbugs:jsr305:3.0.2 jakarta.inject:jakarta.inject-api:2.0.1 javax.inject:javax.inject:1 org.jspecify:jspecify:1.0.0)
procs=$(jars com.google.dagger:hilt-compiler:2.59.2 com.google.dagger:dagger-compiler:2.59.2 com.google.dagger:dagger-spi:2.59.2 \
	com.google.dagger:dagger:2.59.2 com.google.googlejavaformat:google-java-format:1.33.0 com.google.guava:guava:33.0.0-jre \
	com.google.code.findbugs:jsr305:3.0.2 com.google.guava:failureaccess:1.0.2 com.squareup:javapoet:1.13.0 \
	javax.inject:javax.inject:1 net.ltgt.gradle.incap:incap:0.2 org.jetbrains.kotlin:kotlin-metadata-jvm:2.2.20 \
	com.squareup:kotlinpoet:1.11.0 org.jetbrains.kotlin:kotlin-reflect:1.6.10 org.jetbrains.kotlin:kotlin-stdlib-jdk8:1.8.0 \
	org.jetbrains.kotlin:kotlin-stdlib-jdk7:1.8.0 jakarta.inject:jakarta.inject-api:2.0.1 org.jspecify:jspecify:1.0.0 \
	org.checkerframework:checker-compat-qual:2.5.3 org.checkerframework:checker-qual:3.41.0 \
	com.google.errorprone:error_prone_annotations:2.23.0 org.jetbrains:annotations:13.0)
rm -rf "$out/hilt"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes$procs:$out/kc-aa.jar:$deps" RunKspKt "$here/test/hilt/src" "$out/hilt" "$hiltlibs" \
	> "$out/hilt.txt"
rm -rf "$out/hilt/caches" "$out/hilt/classes"
diff -ru "$here/test/hilt/want" "$out/hilt"
echo "hilt: ok"
stock hilt "$here/test/hilt/src" "$hiltlibs" "$procs"
