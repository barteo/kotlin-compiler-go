#!/usr/bin/env bash
# build.sh builds detekt on kc: detekt-cli from the sources of detekt's
# fork, branch $DETEKT_BRANCH of $DETEKT_REPO at $DETEKT_COMMIT (detekt's
# tag $DETEKT_TAG and kc's changes to it, one commit each), compiled with
# kotlinc against kc's Analysis API (jvm/kc-aa) and the Analysis API
# 2.4.21 jar jvm/aa-api builds. kc answers the Analysis API session
# detekt's rules analyze in (docs/detekt.md).
#
#	jvm/detekt/build.sh [test]
#
# It writes build/detekt.jar, build/classpath.txt (the class path run.sh
# runs it with) and, through jvm/kc-aa/build.sh, kc-aa.jar and libkc.so.
# The checkout of detekt is build/detekt, a shallow one of $DETEKT_COMMIT,
# made again when it is at another commit. DETEKT_SRC=DIR builds the
# detekt sources in DIR instead, as they are: changing detekt is editing a
# clone of the fork and building it with DETEKT_SRC, then pushing to
# $DETEKT_BRANCH and setting DETEKT_COMMIT here to its head.
#
# "test" then runs detekt with --all-rules and type resolution over
# test/src and compares its findings, one "file:line:column rule" per line,
# with test/want.txt; DETEKT_UPDATE=1 rewrites want.txt. want.txt is also
# what stock detekt-cli finds there.
#
# Only what detekt-cli runs is built: detekt-api, -core, -parser,
# -psi-utils, -metrics, -tooling, -utils, the rule sets of detekt-rules, the
# reports and the command line. detekt's Gradle build adds two things,
# which build.sh does too: the Poko compiler plugin for detekt-api, and its
# BuildConfig.
set -euo pipefail

DETEKT_REPO=https://github.com/barteo/detekt.git
DETEKT_BRANCH=kc-analysis-api
DETEKT_TAG=v2.0.0-alpha.6
DETEKT_COMMIT=ed862469bccbefd85bc3abc38141eaa435d5433a
MAVEN=${MAVEN_REPO:-https://repo1.maven.org/maven2}

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=$here/build
src=${DETEKT_SRC:-$out/detekt}
mkdir -p "$out/m2"
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}
mode=${1:-}

# The checkout: $DETEKT_COMMIT, shallow.
if [ -z "${DETEKT_SRC:-}" ] && [ "$(git -C "$src" rev-parse -q --verify HEAD 2>/dev/null)" != "$DETEKT_COMMIT" ]; then
	rm -rf "$src"
	git init -q "$src"
	git -C "$src" remote add origin "$DETEKT_REPO"
	git -C "$src" fetch -q --depth 1 origin "$DETEKT_COMMIT"
	git -C "$src" checkout -q --detach "$DETEKT_COMMIT"
fi

# kc's Analysis API: kc-aa.jar and libkc.so, built again if their sources
# are newer.
kcaa=$here/../kc-aa
if [ ! -s "$kcaa/build/kc-aa.jar" ] || [ ! -s "$kcaa/build/libkc.so" ] ||
	[ -n "$(find "$kcaa/src" "$kcaa/gen" "$kcaa/build.sh" "$here/../libkc/src" "$here/../aa-api/build.sh" "$here/../ksp/build.sh" \
		"$root/psi/kinds.txt" -newer "$kcaa/build/kc-aa.jar" -print -quit)" ] ||
	[ -n "$(find "$root" -name '*.go' -not -path '*/build/*' -newer "$kcaa/build/libkc.so" -print -quit)" ]; then
	"$kcaa/build.sh" >/dev/null
fi
lib=$here/../ksp/build/aa/m2
aa=$here/../ksp/build/aa/kc-aa-api-2.4.21.jar
kotlin=$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar
runtime=$kotlin:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar:$lib/caffeine-2.9.3.jar
kotlinc() { java -Xmx3g -cp "$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar" org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -no-stdlib -no-reflect "$@"; }

# fetch PATH downloads PATH from Maven Central into build/m2, retrying on
# the 429s it sends to busy clients.
fetch() {
	local path=$1 dst=$out/m2/$(basename "$1")
	[ -s "$dst" ] && return
	for i in 1 2 3 4 5 6 7 8; do
		curl -sSfL -o "$dst.tmp" "$MAVEN/$path" && mv "$dst.tmp" "$dst" && return
		sleep $((i * 3))
	done
	echo "build.sh: can't download $MAVEN/$path" >&2
	exit 1
}
# jars GROUP:ARTIFACT:VERSION... fetches those jars and prints their class
# path, ":"-prefixed. The versions are those of detekt's
# gradle/libs.versions.toml.
jars() {
	local c g a v cp=
	for c; do
		IFS=: read -r g a v <<< "$c"
		fetch "${g//.//}/$a/$v/$a-$v.jar"
		cp=$cp:$out/m2/$a-$v.jar
	done
	echo "$cp"
}
deps=$(jars org.jcommander:jcommander:1.85 org.snakeyaml:snakeyaml-engine:2.10 org.jetbrains.kotlinx:kotlinx-html-jvm:0.12.0 \
	io.github.detekt.sarif4k:sarif4k-jvm:0.6.0 org.jetbrains.kotlinx:kotlinx-serialization-core-jvm:1.11.0 \
	org.jetbrains.kotlinx:kotlinx-serialization-json-jvm:1.11.0)
poko=$(jars dev.drewhamilton.poko:poko-compiler-plugin:0.23.1 dev.drewhamilton.poko:poko-annotations-jvm:0.23.1)
pokoPlugin=$out/m2/poko-compiler-plugin-0.23.1.jar
pokoAnnotations=$out/m2/poko-annotations-jvm-0.23.1.jar

# detekt's modules, in dependency order.
modules=(
	detekt-utils detekt-parser detekt-psi-utils detekt-api detekt-tooling detekt-metrics detekt-core
	detekt-rules-complexity detekt-rules-coroutines detekt-rules-comments detekt-rules-empty-blocks
	detekt-rules-potential-bugs detekt-rules-exceptions detekt-rules-naming detekt-rules-performance detekt-rules-style
	detekt-report-html detekt-report-markdown detekt-report-sarif detekt-report-checkstyle
	detekt-cli
)
classes=$out/classes
rm -rf "$classes" "$out/gen"
mkdir -p "$classes"
cp="$aa:$runtime:$kcaa/build/kc-aa.jar$deps"
for m in "${modules[@]}"; do
	extra=()
	srcs=$(find "$src/$m/src/main/kotlin" -name '*.kt' | sort)
	if [ "$m" = detekt-api ]; then
		# The BuildConfig detekt's Gradle build generates, and Poko, which
		# writes the equals, hashCode and toString of its @Poko classes.
		version=$(sed -n 's/.*const val DETEKT: String = "\(.*\)"/\1/p' "$src/build-logic/src/main/kotlin/Versions.kt")
		mkdir -p "$out/gen/$m"
		cat > "$out/gen/$m/BuildConfig.kt" <<-EOF
		// Generated by jvm/detekt/build.sh, as detekt's Gradle build generates it.
		package dev.detekt.detekt_api

		internal object BuildConfig {
		    const val DETEKT_VERSION: String = "$version"
		    const val KOTLIN_IMPLEMENTATION_VERSION: String = "2.4.21"
		}
		EOF
		srcs="$srcs $out/gen/$m/BuildConfig.kt"
		extra=(-Xplugin="$pokoPlugin" -P plugin:poko-compiler-plugin:enabled=true
			-P plugin:poko-compiler-plugin:pokoAnnotation=dev/drewhamilton/poko/Poko)
		cp=$cp:$pokoAnnotations
	fi
	echo "build.sh: $m" >&2
	# kotlin-compiler 2.4.21 put the message collector key and the K1
	# API that the psi utilities still use behind opt-ins.
	kotlinc -cp "$cp" -d "$classes/$m" -module-name "$m" -jvm-target 17 -jvm-default=no-compatibility -nowarn \
		-opt-in=org.jetbrains.kotlin.config.MessageCollectorAccess -opt-in=org.jetbrains.kotlin.K1Deprecation \
		"${extra[@]}" $srcs 2>&1 | grep -v '^warning' >&2 || true
	[ -d "$classes/$m" ] && [ -n "$(find "$classes/$m" -name '*.class' -print -quit)" ] ||
		{ echo "build.sh: $m doesn't compile" >&2; exit 1; }
	[ -d "$src/$m/src/main/resources" ] && cp -R "$src/$m/src/main/resources/." "$classes/$m/"
	cp=$cp:$classes/$m
done

# One jar of the modules, their service files joined.
jarRoot=$out/jar
rm -rf "$jarRoot"
mkdir -p "$jarRoot"
for m in "${modules[@]}"; do
	cp -R "$classes/$m/." "$jarRoot/"
done
rm -rf "$jarRoot/META-INF/services"
mkdir -p "$jarRoot/META-INF/services"
for m in "${modules[@]}"; do
	if [ -d "$classes/$m/META-INF/services" ]; then
		for f in "$classes/$m"/META-INF/services/*; do
			cat "$f" >> "$jarRoot/META-INF/services/$(basename "$f")"
			echo >> "$jarRoot/META-INF/services/$(basename "$f")"
		done
	fi
done
rm -f "$out/detekt.jar"
(cd "$jarRoot" && jar cf "$out/detekt.jar" .)
echo "$out/detekt.jar:$kcaa/build/kc-aa.jar:$aa:$runtime$deps$poko" | sed "s|:$pokoPlugin||" > "$out/classpath.txt"
echo "$out/detekt.jar"

[ "$mode" = test ] || exit 0

# The fixture's findings, with the stdlib and the JDK.
rm -rf "$out/test"
mkdir -p "$out/test"
"$here/run.sh" -i "$here/test/src" --analysis-mode full --all-rules -cp "$lib/kotlin-stdlib-2.4.21.jar" \
	--jdk-home "$java_home" -r checkstyle:"$out/test/findings.xml" > "$out/test/run.txt" 2>&1 || true
[ -s "$out/test/findings.xml" ] || { cat "$out/test/run.txt" >&2; echo "build.sh: detekt wrote no report" >&2; exit 1; }
# detekt reports paths relative to the working directory.
python3 -I - "$out/test/findings.xml" > "$out/test/findings.txt" <<-'EOF'
import re, sys, xml.etree.ElementTree as ET
out = []
for f in ET.parse(sys.argv[1]).getroot().iter("file"):
    name = re.sub(r"^.*?test/src/", "", f.get("name"))
    for e in f.iter("error"):
        out.append(f"{name}:{e.get('line')}:{e.get('column')} {e.get('source').split('.')[-1]}")
print("\n".join(sorted(out)))
EOF
if grep -q '^kc: ' "$out/test/run.txt"; then
	grep '^kc: ' "$out/test/run.txt" >&2
fi
if [ "${DETEKT_UPDATE:-}" = 1 ]; then
	cp "$out/test/findings.txt" "$here/test/want.txt"
fi
diff -u "$here/test/want.txt" "$out/test/findings.txt"
echo "detekt: ok ($(wc -l < "$out/test/findings.txt") findings)"
