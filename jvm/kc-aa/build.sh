#!/usr/bin/env bash
# build.sh builds the kc KSP backend: the Analysis API answered by kc
# through libkc (jvm/kc-aa/src), and the libkc.so it loads.
#
#	jvm/kc-aa/build.sh [test]
#
# It writes build/kc-aa.jar and build/libkc.so. With "test" it then runs
# KSP's processing loop on the kc backend over test/slice, with a processor
# that prints the annotated symbols (test/PrintAnnotated.kt), and compares
# the output with test/slice/want.txt.
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
[ -s "$kspwork/ksp-aa-kc.jar" ] || "$here/../ksp/build.sh" "$kspwork" >/dev/null
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
	symbols.KaJavaFieldSymbol
	types.KaUsualClassType types.KaClassErrorType types.KaFunctionType types.KaTypeParameterType
	types.KaFlexibleType types.KaDefinitelyNotNullType
	annotations.KaAnnotation projectStructure.KaSourceModule symbols.pointers.KaSymbolPointer
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

[ "${1:-}" = test ] || exit 0

rm -rf "$out/test-classes" "$out/slice"
kotlinc -cp "$deps:$out/kc-aa.jar" -d "$out/test-classes" -jvm-target 17 -nowarn "$here/test/PrintAnnotated.kt"
LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" \
	-Dkc.stdlib="$lib/kotlin-stdlib-2.4.21.jar" -Didea.home.path="$out" \
	-cp "$out/test-classes:$out/kc-aa.jar:$deps" PrintAnnotatedKt \
	"$here/test/slice/src" "$out/slice" a.Marker a.Other kotlin.Suppress e.Args e.Inner kotlin.Deprecated java.beans.JavaBean \
	decls:kotlin.ReplaceWith decls:kotlin.coroutines.AbstractCoroutineContextKey decls:java.util.AbstractMap.SimpleEntry decls:java.lang.Thread.State \
	decls:f.Strings members:f.Strings members:f.Base members:f.Bag:get,size,toString assign:f.Assign sealed:f.Shape \
	overrides:f.Strings#get:f.Named#get overrides:f.Strings#get:f.Source#get overrides:f.Strings#name:f.Source#get \
	> "$out/slice.txt"
diff -u "$here/test/slice/want.txt" "$out/slice.txt"
echo "slice: ok"
