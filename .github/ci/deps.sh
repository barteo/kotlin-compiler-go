#!/usr/bin/env bash
# Downloads what the tests need besides Go and a JDK: kotlinc, whose
# standard library kc compiles against and whose output testdata/*.out
# was generated with, and the kotlinx.serialization and Compose runtimes
# that TestSerialization and TestCompose run against. The versions are
# the ones testdata was generated with (testdata/gen_expected.sh).
#
# Files go to $KC_DEPS (default ~/.cache/kc-ci), and are downloaded only
# if they aren't there yet. In GitHub Actions the script adds the
# environment the tests read to $GITHUB_ENV and $GITHUB_PATH; elsewhere
# it prints it, so that
#
#   eval "$(.github/ci/deps.sh)"
#
# sets it in the current shell.
set -euo pipefail

KOTLIN=2.4.21
SERIALIZATION=1.9.0
COMPOSE=1.9.2
COLLECTION=1.5.0
COROUTINES=1.8.1

central=https://repo1.maven.org/maven2
google=https://dl.google.com/android/maven2
dir=${KC_DEPS:-$HOME/.cache/kc-ci}
mkdir -p "$dir"

fetch() { # url file
	[ -s "$2" ] && return
	echo "downloading $1" >&2
	curl -fsSL --retry 5 --retry-all-errors -o "$2.part" "$1"
	mv "$2.part" "$2"
}

# kotlinc
if [ ! -x "$dir/kotlinc-$KOTLIN/bin/kotlinc" ]; then
	fetch "https://github.com/JetBrains/kotlin/releases/download/v$KOTLIN/kotlin-compiler-$KOTLIN.zip" "$dir/kotlinc.zip"
	rm -rf "$dir/kotlinc-$KOTLIN" "$dir/unzip"
	unzip -q "$dir/kotlinc.zip" -d "$dir/unzip"
	mv "$dir/unzip/kotlinc" "$dir/kotlinc-$KOTLIN"
	rm -rf "$dir/unzip" "$dir/kotlinc.zip"
fi
kotlin_home=$dir/kotlinc-$KOTLIN

# kotlinx.serialization, for TestSerialization.
ser=()
for a in core json; do
	f=$dir/kotlinx-serialization-$a-jvm-$SERIALIZATION.jar
	fetch "$central/org/jetbrains/kotlinx/kotlinx-serialization-$a-jvm/$SERIALIZATION/kotlinx-serialization-$a-jvm-$SERIALIZATION.jar" "$f"
	ser+=("$f")
done

# Compose's runtime, for TestCompose, in the order it expects: the
# classes.jar of the runtime and runtime-annotation aars, then
# androidx.collection and kotlinx.coroutines.
compose=()
for a in runtime runtime-annotation; do
	f=$dir/compose-$a-$COMPOSE.jar
	if [ ! -s "$f" ]; then
		fetch "$google/androidx/compose/runtime/$a-android/$COMPOSE/$a-android-$COMPOSE.aar" "$dir/$a.aar"
		unzip -p "$dir/$a.aar" classes.jar >"$f.part"
		mv "$f.part" "$f"
		rm "$dir/$a.aar"
	fi
	compose+=("$f")
done
f=$dir/collection-jvm-$COLLECTION.jar
fetch "$google/androidx/collection/collection-jvm/$COLLECTION/collection-jvm-$COLLECTION.jar" "$f"
compose+=("$f")
f=$dir/kotlinx-coroutines-core-jvm-$COROUTINES.jar
fetch "$central/org/jetbrains/kotlinx/kotlinx-coroutines-core-jvm/$COROUTINES/kotlinx-coroutines-core-jvm-$COROUTINES.jar" "$f"
compose+=("$f")

join() { local IFS=:; echo "$*"; }
env=(
	"KOTLIN_HOME=$kotlin_home"
	"KC_SERIALIZATION_CP=$(join "${ser[@]}")"
	"KC_COMPOSE_CP=$(join "${compose[@]}")"
)
if [ -n "${GITHUB_ENV:-}" ]; then
	printf '%s\n' "${env[@]}" >>"$GITHUB_ENV"
	echo "$kotlin_home/bin" >>"$GITHUB_PATH"
else
	printf 'export %q\n' "${env[@]}"
	printf 'export PATH=%q:"$PATH"\n' "$kotlin_home/bin"
fi
