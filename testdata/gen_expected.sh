#!/bin/sh
# Regenerates testdata/run/*.out by compiling each program with kotlinc and
# running it, so kotlinc is the reference for correct behavior. A program
# is a file, X.kt, or a directory, X/, whose main function is in main.kt.
# A first line "// kotlinc: <flags>" in that file passes flags to kotlinc,
# with paths relative to testdata/run.
#
#   testdata/gen_expected.sh [name.kt | name ...]
#
# It also regenerates testdata/lib/X.out for each library test X/: a
# library, X/lib/, compiled with -module-name lib, and a program, X/app/,
# compiled against it, whose main function is in main.kt. A first line
# "// kotlinc: <flags>" in the library's first file, or in main.kt, passes
# flags to kotlinc for it. Arguments of the form lib/X select library
# tests.
#
# And it regenerates testdata/klib/X.out for each kotlinc library test X/,
# whose library kc inlines from kotlinc's classes (TestKotlincLibrary):
# both are compiled with the flags of the first line of the library's
# first file, "// kotlinc: <flags>"; a use/ directory is compiled by
# kotlinc against the app's classes, and run after it. Arguments klib/X
# select them.
#
# And testdata/parcelize.out (TestParcelize): javac compiles parcelize/java/,
# and kotlinc, with its parcelize plugin, the library, parcelize/lib/, the
# program, parcelize/app/, against it, and parcelize/use/ against the
# program, which runs after it. The argument parcelize selects it.
#
# And testdata/serialization.out (TestSerialization): kotlinc, with its
# kotlinx.serialization plugin, compiles serialization/lib/, then
# serialization/app/ against it, and serialization/use/ against the app,
# which runs after it. The runtime is kotlinx.serialization 1.9.0's core
# and JSON jars, from Gradle's cache or $KC_SERIALIZATION_CP. The
# argument serialization selects it.
#
# And testdata/compose.out (TestCompose): javac compiles compose/java/,
# stubs of the Android classes that the Compose runtime uses, and kotlinc,
# with its Compose plugin, compose/lib/, then compose/app/ against it, and
# compose/use/ against the app, which runs. The runtime is Compose's
# runtime-android and runtime-annotation-android 1.9.2, whose classes.jars
# are taken from their aars in Gradle's cache, with androidx.collection
# 1.5.0 and kotlinx.coroutines 1.8.1, or $KC_COMPOSE_CP. The argument
# compose selects it.
#
# And testdata/javasrc.out (TestJavaSources): kotlinc compiles javasrc/,
# Kotlin and Java sources that use each other, and javac then its Java
# against kotlinc's classes. The argument javasrc selects it.
set -eu
root=$(cd "$(dirname "$0")" && pwd)
# kotlinx.coroutines, which kotlinc ships with, for the programs named
# stdlibCoroutines* and the library test coroutines.
home=$(dirname "$(dirname "$(readlink -f "$(command -v kotlinc)")")")
kotlinx=$home/lib/kotlinx-coroutines-core-jvm.jar
if [ ! -f "$kotlinx" ]; then
	kotlinx=$home/libexec/lib/kotlinx-coroutines-core-jvm.jar # Homebrew's
fi
cd "$root/run"
: "${TMPDIR:=/tmp}"
work=$(mktemp -d "$TMPDIR/kc-expected.XXXXXX")
trap 'rm -rf "$work"' EXIT

libs=""
klibs=""
parcelize=""
serialization=""
compose=""
javasrc=""
if [ $# -eq 0 ]; then
	parcelize=1
	serialization=1
	compose=1
	javasrc=1
	libs=$(cd "$root/lib" 2>/dev/null && ls -d */ | sed 's|/$||' || true)
	klibs=$(cd "$root/klib" 2>/dev/null && ls -d */ | sed 's|/$||' || true)
	set -- *.kt
	for d in */; do
		if [ -f "$d/main.kt" ]; then
			set -- "$@" "${d%/}"
		fi
	done
fi
runs=""
for f in "$@"; do
	case $f in
	lib/*) libs="$libs ${f#lib/}" ;;
	klib/*) klibs="$klibs ${f#klib/}" ;;
	parcelize) parcelize=1 ;;
	serialization) serialization=1 ;;
	compose) compose=1 ;;
	javasrc) javasrc=1 ;;
	*) runs="$runs $f" ;;
	esac
done
# shellcheck disable=SC2086 # names are words
set -- $runs
for f in "$@"; do
	name=${f%.kt}
	name=${name%/}
	out="$work/$name"
	mkdir -p "$out"
	if [ -d "$name" ]; then
		main="$name/main.kt"
		base=main
	else
		main="$f"
		base=$name
	fi
	flags=$(sed -n '1s|^// kotlinc: ||p' "$main")
	cp=""
	case $base in
	stdlibCoroutines*) cp=$kotlinx ;;
	esac
	if [ -d "$name" ]; then
		# The files one by one: -Xcommon-sources only matches them so.
		srcs=$(find "$name" -name '*.kt' | sort)
	else
		srcs=$f
	fi
	# shellcheck disable=SC2086 # flags and srcs are words
	kotlinc -nowarn $flags ${cp:+-cp "$cp"} $srcs -d "$out" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# Facade class name: first letter upper-cased, plus Kt, in the file's
	# package.
	facade="$(printf '%s' "$base" | cut -c1 | tr '[:lower:]' '[:upper:]')$(printf '%s' "$base" | cut -c2-)Kt"
	pkg=$(sed -n 's/^package \([A-Za-z0-9_.]*\).*/\1/p' "$main" | head -1)
	if [ -n "$pkg" ]; then
		facade="$pkg.$facade"
	fi
	kotlin -cp "$out${cp:+:$cp}" "$facade" >"$name.out" 2>/dev/null
	echo "wrote $name.out"
done

# Library tests.
cd "$root/lib"
for name in $libs; do
	name=${name%/}
	out="$work/lib-$name"
	mkdir -p "$out/lib" "$out/app"
	cp=""
	if [ "$name" = coroutines ]; then
		cp=":$kotlinx"
	fi
	libflags=$(sed -n '1s|^// kotlinc: ||p' "$(find "$name/lib" -name '*.kt' | sort | head -n 1)")
	appflags=$(sed -n '1s|^// kotlinc: ||p' "$name/app/main.kt")
	# shellcheck disable=SC2046,SC2086 # file names and flags are words
	kotlinc -nowarn -module-name lib $libflags ${cp:+-cp "${cp#:}"} $(find "$name/lib" -name '*.kt' | sort) -d "$out/lib" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046,SC2086
	kotlinc -nowarn $appflags -cp "$out/lib$cp" $(find "$name/app" -name '*.kt' | sort) -d "$out/app" 2>"$work/err" || { cat "$work/err"; exit 1; }
	kotlin -cp "$out/app:$out/lib$cp" MainKt >"$name.out" 2>/dev/null
	echo "wrote lib/$name.out"
done

# kotlinc library tests.
cd "$root/klib"
for name in $klibs; do
	name=${name%/}
	out="$work/klib-$name"
	mkdir -p "$out/lib" "$out/app"
	flags=$(sed -n '1s|^// kotlinc: ||p' "$(find "$name/lib" -name '*.kt' | sort | head -1)")
	# shellcheck disable=SC2046,SC2086 # flags and file names are words
	kotlinc -nowarn $flags -module-name lib $(find "$name/lib" -name '*.kt' | sort) -d "$out/lib" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# An app whose first file starts with "// friend" sees the library's
	# internal declarations.
	friend=""
	if [ "$(head -1 "$(find "$name/app" -name '*.kt' | sort | head -1)")" = "// friend" ]; then
		friend="-Xfriend-paths=$out/lib"
	fi
	# shellcheck disable=SC2046,SC2086
	kotlinc -nowarn $flags $friend -cp "$out/lib" $(find "$name/app" -name '*.kt' | sort) -d "$out/app" 2>"$work/err" || { cat "$work/err"; exit 1; }
	kotlin -cp "$out/app:$out/lib" MainKt >"$name.out" 2>/dev/null
	# A use/ module is compiled against the app's classes alone, without
	# the library, and runs after it.
	if [ -d "$name/use" ]; then
		mkdir -p "$out/use"
		# shellcheck disable=SC2046
		kotlinc -nowarn -cp "$out/app" $(find "$name/use" -name '*.kt' | sort) -d "$out/use" 2>"$work/err" || { cat "$work/err"; exit 1; }
		kotlin -cp "$out/use:$out/app:$out/lib" UseKt >>"$name.out" 2>/dev/null
	fi
	echo "wrote klib/$name.out"
done

# The parcelize test.
if [ -n "$parcelize" ]; then
	cd "$root/parcelize"
	klib=$(dirname "$kotlinx")
	out="$work/parcelize"
	mkdir -p "$out/java" "$out/lib" "$out/app" "$out/use"
	plugin="-Xplugin=$klib/parcelize-compiler.jar"
	# shellcheck disable=SC2046 # file names are words
	javac -d "$out/java" $(find java -name '*.java' | sort)
	# shellcheck disable=SC2046
	kotlinc -nowarn -module-name lib "$plugin" -cp "$out/java:$klib/parcelize-runtime.jar" $(find lib -name '*.kt' | sort) -d "$out/lib" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	kotlinc -nowarn "$plugin" -cp "$out/java:$klib/parcelize-runtime.jar:$out/lib" $(find app -name '*.kt' | sort) -d "$out/app" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	kotlinc -nowarn "$plugin" -cp "$out/java:$klib/parcelize-runtime.jar:$out/app" $(find use -name '*.kt' | sort) -d "$out/use" 2>"$work/err" || { cat "$work/err"; exit 1; }
	kotlin -cp "$out/app:$out/lib:$out/java" MainKt >"$root/parcelize.out" 2>/dev/null
	kotlin -cp "$out/use:$out/app:$out/lib:$out/java" UseKt >>"$root/parcelize.out" 2>/dev/null
	echo "wrote parcelize.out"
fi

# The serialization test.
if [ -n "$serialization" ]; then
	cd "$root/serialization"
	klib=$(dirname "$kotlinx")
	rt=${KC_SERIALIZATION_CP:-}
	if [ -z "$rt" ]; then
		cache=$HOME/.gradle/caches/modules-2/files-2.1/org.jetbrains.kotlinx
		for a in core json; do
			jar=$(find "$cache/kotlinx-serialization-$a-jvm/1.9.0" -name "kotlinx-serialization-$a-jvm-1.9.0.jar" 2>/dev/null | head -1)
			if [ -z "$jar" ]; then
				echo "kotlinx.serialization 1.9.0 not found: set KC_SERIALIZATION_CP" >&2
				exit 1
			fi
			rt=${rt:+$rt:}$jar
		done
	fi
	out="$work/serialization"
	mkdir -p "$out/lib" "$out/app" "$out/use"
	plugin="-Xplugin=$klib/kotlinx-serialization-compiler-plugin.jar"
	# shellcheck disable=SC2046 # file names are words
	kotlinc -nowarn -module-name lib "$plugin" -cp "$rt" $(find lib -name '*.kt' | sort) -d "$out/lib" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	kotlinc -nowarn -module-name app "$plugin" -cp "$rt:$out/lib" $(find app -name '*.kt' | sort) -d "$out/app" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	kotlinc -nowarn "$plugin" -cp "$rt:$out/lib:$out/app" $(find use -name '*.kt' | sort) -d "$out/use" 2>"$work/err" || { cat "$work/err"; exit 1; }
	kotlin -cp "$out/app:$out/lib:$rt" app.MainKt >"$root/serialization.out" 2>/dev/null
	kotlin -cp "$out/use:$out/app:$out/lib:$rt" UseKt >>"$root/serialization.out" 2>/dev/null
	echo "wrote serialization.out"
fi

# The compose test.
if [ -n "$compose" ]; then
	cd "$root/compose"
	klib=$(dirname "$kotlinx")
	out="$work/compose"
	mkdir -p "$out/java" "$out/lib" "$out/app" "$out/use"
	rt=${KC_COMPOSE_CP:-}
	if [ -z "$rt" ]; then
		cache=$HOME/.gradle/caches/modules-2/files-2.1
		aar=$(find "$cache/androidx.compose.runtime/runtime-android/1.9.2" -name runtime.aar 2>/dev/null | head -1)
		annotations=$(find "$cache/androidx.compose.runtime/runtime-annotation-android/1.9.2" -name runtime-annotation.aar 2>/dev/null | head -1)
		collection=$(find "$cache/androidx.collection/collection-jvm/1.5.0" -name collection-jvm-1.5.0.jar 2>/dev/null | head -1)
		coroutines=$(find "$cache/org.jetbrains.kotlinx/kotlinx-coroutines-core-jvm/1.8.1" -name kotlinx-coroutines-core-jvm-1.8.1.jar 2>/dev/null | head -1)
		if [ -z "$aar" ] || [ -z "$annotations" ] || [ -z "$collection" ] || [ -z "$coroutines" ]; then
			echo "Compose's runtime 1.9.2 not found: set KC_COMPOSE_CP" >&2
			exit 1
		fi
		unzip -p "$aar" classes.jar >"$out/runtime.jar"
		unzip -p "$annotations" classes.jar >"$out/runtime-annotation.jar"
		rt=$out/runtime.jar:$out/runtime-annotation.jar:$collection:$coroutines
	fi
	plugin="-Xplugin=$klib/compose-compiler-plugin.jar"
	# shellcheck disable=SC2046 # file names are words
	javac -d "$out/java" $(find java -name '*.java' | sort)
	# shellcheck disable=SC2046
	kotlinc -nowarn -module-name lib "$plugin" -cp "$rt" $(find lib -name '*.kt' | sort) -d "$out/lib" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	kotlinc -nowarn -module-name app "$plugin" -cp "$rt:$out/lib" $(find app -name '*.kt' | sort) -d "$out/app" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	kotlinc -nowarn "$plugin" -cp "$rt:$out/lib:$out/app" $(find use -name '*.kt' | sort) -d "$out/use" 2>"$work/err" || { cat "$work/err"; exit 1; }
	kotlin -cp "$out/use:$out/app:$out/lib:$out/java:$rt" UseKt >"$root/compose.out" 2>/dev/null
	echo "wrote compose.out"
fi

# The Java sources test.
if [ -n "$javasrc" ]; then
	cd "$root/javasrc"
	out="$work/javasrc"
	mkdir -p "$out"
	stdlib=$(dirname "$kotlinx")/kotlin-stdlib.jar
	# shellcheck disable=SC2046 # file names are words
	kotlinc -nowarn $(find . -name '*.kt' -o -name '*.java' | sort) -d "$out" 2>"$work/err" || { cat "$work/err"; exit 1; }
	# shellcheck disable=SC2046
	javac -cp "$out:$stdlib" -d "$out" $(find . -name '*.java' | sort)
	kotlin -cp "$out" MainKt >"$root/javasrc.out" 2>/dev/null
	echo "wrote javasrc.out"
fi
