#!/usr/bin/env bash
# build.sh builds detekt on kc: a stand-in for the Analysis API's
# standalone session (jvm/kc-detekt/src) whose KaSession kc answers with
# the facts of package analysis, read through libkc (internal/lintops),
# and the libkc.so it loads.
#
#	jvm/kc-detekt/build.sh
#
# It writes build/kc-detekt.jar and build/libkc.so. detekt runs on kc
# unchanged: its own class path, with kc-detekt.jar in place of
# detekt-kotlin-analysis-api-standalone (see run.sh).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=$here/build
mkdir -p "$out"
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}

# The compiler that builds kc-detekt: kotlin-compiler 2.4.21, as
# jvm/aa-api/build.sh downloads it.
aawork=$here/../ksp/build/aa
[ -s "$aawork/m2/kotlin-compiler-2.4.21.jar" ] || aawork=$here/../aa-api/build
[ -s "$aawork/m2/kotlin-compiler-2.4.21.jar" ] || "$here/../aa-api/build.sh" "$aawork" >/dev/null
tl=$aawork/m2
runtime=$tl/kotlin-compiler-2.4.21.jar:$tl/kotlin-stdlib-2.4.21.jar:$tl/kotlin-reflect-2.4.21.jar:$tl/kotlinx-coroutines-core-jvm-1.8.0.jar:$tl/annotations-23.0.0.jar
# kc-detekt is compiled against the Analysis API and kotlin-compiler that
# detekt ships, which it runs with: DETEKT_HOME is a detekt-cli
# distribution (detekt-cli/build/install/detekt-cli).
: "${DETEKT_HOME:?set DETEKT_HOME to a detekt-cli distribution}"
dl=$DETEKT_HOME/lib
aa=$(ls "$dl"/detekt-kotlin-analysis-api-[0-9]*.jar)
deps=$aa:$(ls "$dl"/kotlin-compiler-*.jar):$(ls "$dl"/kotlin-stdlib-[0-9]*.jar):$(ls "$dl"/caffeine-*.jar)
kotlinc() { java -Xmx3g -cp "$runtime" org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -no-stdlib -no-reflect "$@"; }

optins="-opt-in=org.jetbrains.kotlin.K1Deprecation -opt-in=org.jetbrains.kotlin.config.CompilerConfiguration.Internals -opt-in=org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi -opt-in=org.jetbrains.kotlin.psi.KtExperimentalApi"
for m in KaImplementationDetail KaExperimentalApi KaNonPublicApi KaIdeApi KaPlatformInterface KaContextParameterApi KaSpiExtensionPoint; do
	optins="$optins -opt-in=org.jetbrains.kotlin.analysis.api.$m"
done

# libkc.so.
(cd "$root" && CGO_CFLAGS="-I$java_home/include -I$java_home/include/linux" \
	go build -tags libkc -buildmode=c-shared -o "$out/libkc.so" ./cmd/libkc)

# The stubs of the Analysis API classes kc implements.
stubbed=(
	symbols.KaNamedClassSymbol symbols.KaNamedFunctionSymbol symbols.KaKotlinPropertySymbol
	symbols.KaConstructorSymbol symbols.KaValueParameterSymbol symbols.KaTypeParameterSymbol
	symbols.KaLocalVariableSymbol symbols.KaAnonymousFunctionSymbol symbols.KaAnonymousObjectSymbol
	symbols.KaPropertyGetterSymbol symbols.KaPropertySetterSymbol symbols.KaReceiverParameterSymbol
	symbols.KaEnumEntrySymbol symbols.KaJavaFieldSymbol symbols.KaTypeAliasSymbol symbols.KaSyntheticJavaPropertySymbol
	symbols.KaDestructuringDeclarationSymbol symbols.KaBackingFieldSymbol
	types.KaUsualClassType types.KaFunctionType types.KaTypeParameterType types.KaFlexibleType
	types.KaClassErrorType types.KaDefinitelyNotNullType types.KaTypeArgumentWithVariance types.KaStarTypeProjection
	resolution.KaSimpleFunctionCall resolution.KaSimpleVariableAccessCall resolution.KaPartiallyAppliedSymbol
	resolution.KaSuccessCallInfo resolution.KaExplicitReceiverValue resolution.KaImplicitReceiverValue
	resolution.KaSmartCastedReceiverValue resolution.KaCompoundVariableAccessCall resolution.KaDelegatedConstructorCall
	resolution.KaAnnotationCall
	signatures.KaFunctionSignature signatures.KaVariableSignature
	annotations.KaAnnotationList annotations.KaAnnotation scopes.KaScope
	projectStructure.KaSourceModule projectStructure.KaLibraryModule symbols.pointers.KaSymbolPointer
)
rm -rf "$out/gen"
kotlinc -cp "$runtime" -d "$out/gen" "$here/gen/GenStubs.kt" 2>&1 | grep -v '^warning' || true
java -cp "$out/gen:$tl/kotlin-reflect-2.4.21.jar:$deps" GenStubsKt "$out/Stubs.kt" io.github.barteo.kc.detekt \
	$(printf 'org.jetbrains.kotlin.analysis.api.%s ' "${stubbed[@]}")
[ "${1:-}" = stubs ] && exit 0

# The session.
classes=$out/classes
rm -rf "$classes"
mkdir -p "$classes"
javac -d "$classes" $(find "$here/../libkc/src" -name '*.java')
kotlinc -cp "$deps:$classes" -d "$classes" -module-name kc-detekt -jvm-target 17 -jvm-default=enable \
	-Xcontext-parameters -Xskip-metadata-version-check $optins -nowarn "$out/Stubs.kt" $(find "$here/src" -name '*.kt')
mkdir -p "$classes/io/github/barteo/kc/detekt"
cp "$root/psi/kinds.txt" "$classes/io/github/barteo/kc/detekt/kinds.txt"
rm -f "$out/kc-detekt.jar"
(cd "$classes" && jar cf "$out/kc-detekt.jar" .)
echo "$out/kc-detekt.jar"
