#!/usr/bin/env bash
# build.sh builds the kc KSP backend: the Analysis API answered by kc
# through libkc (jvm/kc-aa/src), and the libkc.so it loads.
#
#	jvm/kc-aa/build.sh [test|stock|ksp|members|kotlinpoet]
#
# It writes build/kc-aa.jar and build/libkc.so. With "test" it then runs
# KSP's processing loop on the kc backend over test/slice, with a processor
# that prints the annotated symbols (test/PrintAnnotated.kt), and compares
# the output with test/slice/want.txt; then it runs Moshi's KSP codegen over
# test/moshi/src (test/RunKsp.kt) and compares what it generates with
# test/moshi/want, and Dagger's, Room's and Hilt's the same way, over
# test/{dagger,room,hilt}/src, and checks incremental processing's dirty
# set over test/ic. "stock" does what "test" does, and then runs
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
# "kotlinpoet" runs kotlinpoet's interop:ksp tests on kc and on stock KSP
# (test/kotlinpoet/Compilation.kt, docs/analysis-api.md step 20).
#
# "members" writes members.txt: each Analysis API member the KSP fork's
# bytecode uses, and whether kc answers it (gen/Members.kt).
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
	symbols.KaDestructuringDeclarationSymbol symbols.KaSyntheticJavaPropertySymbol symbols.KaAnonymousObjectSymbol symbols.KaLocalVariableSymbol
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
cp "$root/psi/kinds.txt" "$classes/io/github/barteo/kc/aa/kinds.txt"
rm -f "$out/kc-aa.jar"
(cd "$classes" && jar cf "$out/kc-aa.jar" .)
echo "$out/kc-aa.jar"

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

mode=${1:-}
if [ "$mode" = members ]; then
	# members.txt: the Analysis API members the KSP fork uses, and which kc answers.
	rm -rf "$out/members"
	kotlinc -cp "$runtime" -d "$out/members" "$here/gen/Members.kt" 2>&1 | grep -v '^warning' || true
	java -cp "$out/members:$out/kc-aa.jar:$deps" MembersKt "$here/members.txt" "$kspwork/ksp-aa-kc.jar" "$out/kc-aa.jar"
	grep '^# total' "$here/members.txt"
	exit 0
fi
# kotlinpoet's interop:ksp tests (TestProcessorTest, KsTypesTest), from
# its sources at KOTLINPOET_TAG, on a stand-in for kotlin-compile-testing
# (test/kotlinpoet/Compilation.kt) that runs KSP alone: on kc, then on
# stock KSP 2.3.6.
if [ "$mode" = kotlinpoet ]; then
	tag=${KOTLINPOET_TAG:-2.4.0}
	kp=$out/kotlinpoet
	[ -d "$kp/src-$tag" ] || git clone -q --depth 1 -b "$tag" https://github.com/square/kotlinpoet.git "$kp/src-$tag"
	t=$kp/src-$tag/interop/ksp/test-processor/src
	poet=$(jars com.squareup:kotlinpoet-jvm:$tag com.squareup:kotlinpoet-ksp:$tag)
	junit=$(jars junit:junit:4.13.2 org.hamcrest:hamcrest-core:1.3 com.willowtreeapps.assertk:assertk-jvm:0.28.1 \
		com.willowtreeapps.opentest4k:opentest4k-jvm:1.3.0 org.opentest4j:opentest4j:1.3.0 \
		org.jetbrains.kotlin:kotlin-test:2.4.21 org.jetbrains.kotlin:kotlin-test-junit:2.4.21)
	stockcp=$(jars com.google.devtools.ksp:symbol-processing-aa-embeddable:2.3.6 com.google.devtools.ksp:symbol-processing-api:2.3.6 \
		com.google.devtools.ksp:symbol-processing-common-deps:2.3.6 org.jetbrains.kotlin:kotlin-stdlib:2.3.0)
	stockcp=$stockcp:$(jars org.jetbrains.intellij.deps.kotlinx:kotlinx-coroutines-core-jvm:1.8.0-intellij-14)
	rm -rf "$kp/proc" "$kp/tests" "$kp/shim-kc" "$kp/shim-stock" "$kp/work"
	kotlinc -cp "$deps$poet$(jars com.google.auto.service:auto-service-annotations:1.1.1)" -d "$kp/proc" -jvm-target 17 -nowarn \
		$(find "$t/main" -name '*.kt')
	kotlinc -cp "$deps$poet$junit:$kp/proc" -d "$kp/shim-kc" -jvm-target 17 -nowarn "$here/test/kotlinpoet/Compilation.kt"
	kotlinc -cp "${stockcp#:}" -language-version 2.3 -api-version 2.3 -d "$kp/shim-stock" -jvm-target 17 -nowarn \
		"$here/test/kotlinpoet/Compilation.kt"
	kotlinc -cp "$deps$poet$junit:$kp/proc:$kp/shim-kc" -d "$kp/tests" -jvm-target 17 -nowarn $(find "$t/test" -name '*.kt')
	classes="com.squareup.kotlinpoet.ksp.test.processor.TestProcessorTest com.squareup.kotlinpoet.ksp.test.processor.KsTypesTest"
	sources=$lib/kotlin-stdlib-2.4.21.jar:$kp/proc${poet}
	mkdir -p "$kp/work"
	(cd "$kp/work" && LD_PRELOAD=$java_home/lib/libjsig.so java -Xss8m ${KSP_JAVA_OPTS:-} -Dksp.backend=kc -Dkc.lib="$out/libkc.so" \
		-Didea.home.path="$out" -Dkotlinpoet.classpath="$sources" \
		-cp "$kp/shim-kc:$kp/tests:$kp/proc:$out/kc-aa.jar:$deps$poet$junit" org.junit.runner.JUnitCore $classes > "$kp/kc.txt" 2>&1) || true
	(cd "$kp/work" && java -Dkotlinpoet.classpath="$sources" -cp "$kp/shim-stock:$kp/tests:$kp/proc:$lib/kotlin-stdlib-2.4.21.jar${stockcp}${poet}${junit}" \
		org.junit.runner.JUnitCore $classes > "$kp/stock.txt" 2>&1) || true
	for b in kc stock; do
		echo "kotlinpoet: $b: $(grep -E '^(OK|Tests run)' "$kp/$b.txt")"
		grep -E '^[0-9]+\) ' "$kp/$b.txt" | sed "s/^/kotlinpoet: $b: /"
	done
	exit 0
fi
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
# Incremental processing (icrun): Moshi's codegen over a copy of
# test/ic/src, then, test/ic/edit copied over it, again with KSP told which
# files were modified. The second run's dirty set, from KSP's log, must be
# test/ic/dirty.txt, and its outputs those of a clean run over the edited
# sources. On stock KSP (in "stock" mode) the dirty set differs from
# dirty.txt only as recorded in test/ic/stock.diff (docs/analysis-api.md,
# step 16).
iclibs=$lib/kotlin-stdlib-2.4.21.jar:$out/m2/moshi-1.15.2.jar:$out/m2/okio-jvm-3.7.0.jar
# icrun DIR JAVA... runs the test in DIR with the command JAVA... (a java
# with its options and class path) and writes DIR/dirty.txt. With
# icclean=1, the outputs (all of DIR/out but its caches) are deleted before
# the second run, as Gradle does, so that KSP restores the clean ones from
# its backups.
icrun() {
	local d=$1 f mod=
	shift
	rm -rf "$d"
	mkdir -p "$d"
	cp -r "$here/test/ic/src" "$d/src"
	"$@" -Dksp.incremental=true -Dksp.incrementalLog=true RunKspKt "$d/src" "$d/out" "$iclibs" > "$d/first.txt"
	grep -q '^Dirty / All: 100.00%$' "$d/out/caches/logs/kspDirtySet.log"
	cp -r "$here/test/ic/edit/." "$d/src"
	if [ "${icclean:-}" = 1 ]; then
		find "${d:?}/out" -mindepth 1 -maxdepth 1 ! -name caches -exec rm -r {} +
	fi
	for f in $(cd "$here/test/ic/edit" && find . -type f | sort); do
		mod=$mod:$d/src/${f#./}
	done
	"$@" -Dksp.incremental=true -Dksp.incrementalLog=true -Dksp.modified="${mod#:}" RunKspKt "$d/src" "$d/out" "$iclibs" \
		> "$d/second.txt"
	awk '/^=== Build/ { n = 0; delete s } /^Dirty:$/ { d = 1; next } d && /^ / { s[n++] = $1; next } { d = 0 }
		END { for (i = 0; i < n; i++) print s[i] }' "$d/out/caches/logs/kspDirtySet.log" | sort > "$d/dirty.txt"
	"$@" RunKspKt "$d/src" "$d/clean" "$iclibs" > "$d/clean.txt"
	rm -rf "$d/out/caches" "$d/clean/caches"
	diff -ru "$d/clean" "$d/out"
}
icrun "$out/ic" env LD_PRELOAD="$java_home/lib/libjsig.so" java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes:$out/kc-aa.jar:$deps$procs"
diff -u "$here/test/ic/dirty.txt" "$out/ic/dirty.txt"
echo "incremental: ok ($(wc -l < "$out/ic/dirty.txt") of $(find "$here/test/ic/src" -type f | wc -l) files dirty)"
icclean=1 icrun "$out/ic-gradle" env LD_PRELOAD="$java_home/lib/libjsig.so" java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" \
	-Didea.home.path="$out" -cp "$out/runksp-classes:$out/kc-aa.jar:$deps$procs"
diff -u "$here/test/ic/dirty.txt" "$out/ic-gradle/dirty.txt"
echo "incremental, outputs deleted: ok ($(wc -l < "$out/ic-gradle/dirty.txt") of $(find "$here/test/ic/src" -type f | wc -l) files dirty)"
if [ "$mode" = stock ]; then
	icrun "$out/stock-ic" java -Dksp.languageVersion=2.3 -cp "$out/runksp-stock$procs$stockcp"
	{ diff "$here/test/ic/dirty.txt" "$out/stock-ic/dirty.txt" || true; } | grep '^[<>]' > "$out/stock-ic.diff" || true
	diff -u "$here/test/ic/stock.diff" "$out/stock-ic.diff"
	echo "incremental: stock KSP differs as recorded"
fi
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
# resolves for room-compiler 2.8.4 (room-jars.sh) and hilt-compiler
# 2.59.2 (hilt-jars.sh), but for KSP's API and the Kotlin standard
# library, which KSP provides.
. "$here/room-jars.sh"
roomlibs=$lib/kotlin-stdlib-2.4.21.jar$(jars "${room_libs[@]}")
procs=$(jars "${room_procs[@]}")
serialization=$(jars "${room_serialization[@]}")
rm -rf "$out/room"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes$procs:$out/kc-aa.jar:$deps" RunKspKt "$here/test/room/src" "$out/room" "$roomlibs" \
	> "$out/room.txt"
rm -rf "$out/room/caches" "$out/room/classes"
diff -ru "$here/test/room/want" "$out/room"
echo "room: ok"
stock room "$here/test/room/src" "$roomlibs" "$procs$serialization"

. "$here/hilt-jars.sh"
hiltlibs=$lib/kotlin-stdlib-2.4.21.jar$(jars "${hilt_libs[@]}")
procs=$(jars "${hilt_procs[@]}")
rm -rf "$out/hilt"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
	-cp "$out/runksp-classes$procs:$out/kc-aa.jar:$deps" RunKspKt "$here/test/hilt/src" "$out/hilt" "$hiltlibs" \
	> "$out/hilt.txt"
rm -rf "$out/hilt/caches" "$out/hilt/classes"
diff -ru "$here/test/hilt/want" "$out/hilt"
echo "hilt: ok"
stock hilt "$here/test/hilt/src" "$hiltlibs" "$procs"
