#!/usr/bin/env bash
# gradle.sh runs the KSP tasks of Gradle builds on kc, through KSP's Gradle
# plugin and tools/gradle/use-kc.init.gradle.kts with KC_KSP, and compares
# what the processors generate with what they generate on stock KSP. It
# needs jvm/kc-aa/build.sh to have run.
#
#	jvm/kc-aa/gradle.sh [BUILDS]
#
# It generates two projects of Kotlin/JVM modules whose sources are those
# of test/{moshi,dagger,room}/src: a module for each of Moshi's codegen,
# Dagger's processor and Room's, and one with all three. Dagger brings a
# Guava and Room a kotlinx.serialization of their own, which are not the
# ones kotlin-compiler bundles and the KSP fork uses; Room exports its
# schema, which it writes with that kotlinx.serialization. Project 1 has KSP's
# Gradle plugin 2.3.13; project 2 has 2.3.12 and the configuration cache,
# with which the plugin closes KSP's class loaders when a build ends.
#
# One daemon runs every build. First each project's kspKotlin tasks on
# stock KSP, whose output is the reference. Then BUILDS times (default 3)
# project 1's and project 2's in turn on kc, with KC_STRICT=1: a class
# loader for each project's KSP class path, and for project 2 a new one in
# each build, all on the one libkc. Then, with a copy of the KC_KSP
# directory that lacks kc-aa.jar: under KC_STRICT=1 the build fails, and
# without it stock KSP takes over and the output is the reference's. Then,
# with KC_VARIANTS=debug, which names no variant of these modules, KSP
# stays stock. Then, with KC_KSP_KEEP=1, two builds of each project: the
# first loads the processors of its four class paths or finds those the
# other project loaded, and the second finds all four kept. Then, with
# KC_KSP_FORGET=1, two builds of each project, whose four runs each keep
# nothing for the next. Last, with a component in project 1 that Dagger
# finds no binding for: the build fails and reports Dagger's error once,
# both when stock KSP runs after kc and under KC_STRICT=1.
#
# Environment:
#	GRADLE  the gradle command (default gradle)
#	GRADLE_USER_HOME  Gradle's user home (default build/gradle-test/home,
#	        so the daemon is this script's own, which it ends by its pid)
set -euo pipefail

builds=${1:-3}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=$here/build
work=$out/gradle-test
ksp=$out/kc-ksp
init=$root/tools/gradle/use-kc.init.gradle.kts
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}
export GRADLE_USER_HOME=${GRADLE_USER_HOME:-$work/home}
[ -s "$ksp/kc-ksp-loader.jar" ] || { echo "gradle.sh: run jvm/kc-aa/build.sh first" >&2; exit 1; }

rm -rf "$work"/project* "$work/stock" "$work/broken" "$work"/*.log
mkdir -p "$work" "$GRADLE_USER_HOME"
modules="moshi dagger room all"
# project K KSP_VERSION CONFIGURATION_CACHE
project() {
	local p=$work/project$1 m s room
	mkdir -p "$p"
	cat > "$p/settings.gradle.kts" <<-EOF
	pluginManagement { repositories { mavenCentral(); gradlePluginPortal() } }
	dependencyResolutionManagement { repositories { mavenCentral(); google() } }
	rootProject.name = "kc-ksp-gradle-$1"
	include(":moshi", ":dagger", ":room", ":all")
	EOF
	cat > "$p/build.gradle.kts" <<-EOF
	plugins {
	    kotlin("jvm") version "2.4.21" apply false
	    id("com.google.devtools.ksp") version "$2" apply false
	}
	EOF
	cat > "$p/gradle.properties" <<-EOF
	org.gradle.jvmargs=-Xmx3g
	org.gradle.parallel=true
	org.gradle.configuration-cache=$3
	EOF
	for m in $modules; do
		room=
		mkdir -p "$p/$m/src/main/kotlin"
		{
			echo 'plugins {'
			echo '    kotlin("jvm")'
			echo '    id("com.google.devtools.ksp")'
			echo '}'
			echo 'dependencies {'
			for s in moshi dagger room; do
				[ "$m" = "$s" ] || [ "$m" = all ] || continue
				cp -r "$here/test/$s/src/." "$p/$m/src/main/kotlin/"
				case $s in
				moshi)
					echo '    implementation("com.squareup.moshi:moshi:1.15.2")'
					echo '    ksp("com.squareup.moshi:moshi-kotlin-codegen:1.15.2")'
					;;
				dagger)
					echo '    implementation("com.google.dagger:dagger:2.59.2")'
					echo '    ksp("com.google.dagger:dagger-compiler:2.59.2")'
					;;
				room)
					echo '    implementation("androidx.room:room-runtime:2.8.4")'
					echo '    ksp("androidx.room:room-compiler:2.8.4")'
					sed -i 's/exportSchema = false/exportSchema = true/' "$p/$m/src/main/kotlin/r/Db.kt"
					room='ksp { arg("room.schemaLocation", layout.buildDirectory.dir("schemas").get().asFile.path) }'
					;;
				esac
			done
			echo '}'
			echo "$room"
		} > "$p/$m/build.gradle.kts"
	done
}
project 1 2.3.13 false
project 2 2.3.12 true

# The daemons of this user home, by pid.
daemons() {
	local f
	for f in "$GRADLE_USER_HOME"/daemon/*/daemon-*.out.log; do
		[ -e "$f" ] || continue
		f=${f##*daemon-}
		kill -0 "${f%.out.log}" 2>/dev/null && echo "${f%.out.log}"
	done
	return 0
}
before=$(daemons)
stop() {
	local pid
	for pid in $(daemons); do
		grep -qx "$pid" <<< "$before" || kill "$pid" 2>/dev/null || true
	done
}
trap stop EXIT

n=0
# g K ARGS... runs project K's kspKotlin tasks; its log is $log.
g() {
	local k=$1
	shift
	n=$((n + 1))
	log=$work/build-$n.log
	${GRADLE:-gradle} --daemon --console=plain -p "$work/project$k" kspKotlin --rerun-tasks "$@" > "$log" 2>&1
}
# generated K DIR copies what project K's processors generated to DIR.
generated() {
	local m
	rm -rf "$2"
	for m in $modules; do
		mkdir -p "$2/$m"
		cp -r "$work/project$1/$m/build/generated/ksp/." "$2/$m/"
		[ ! -d "$work/project$1/$m/build/schemas" ] || cp -r "$work/project$1/$m/build/schemas" "$2/$m/"
	done
}
# same K WHAT compares project K's generated files with stock KSP's.
same() {
	generated "$1" "$work/got"
	diff -r "$work/stock/$1" "$work/got" > "$work/diff.txt" || { echo "gradle.sh: $2: differs from stock KSP:" >&2; head -40 "$work/diff.txt" >&2; exit 1; }
}
# on_kc N: the build ran N KSP tasks on kc.
on_kc() {
	local got
	got=$(grep -c '^kc: KSP on .*: [0-9]* ms$' "$log" || true)
	[ "$got" = "$1" ] || { echo "gradle.sh: $got KSP tasks ran on kc, not $1: $log" >&2; exit 1; }
}

# A first build fetches the jars, retrying on the 429s Maven Central sends
# to busy clients.
for k in 1 2; do
	for i in 1 2 3 4 5 6; do
		g "$k" && break
		grep -q 'status code 429' "$log" || { cat "$log" >&2; exit 1; }
		sleep $((i * 10))
	done
	on_kc 0
	generated "$k" "$work/stock/$k"
	echo "stock: project $k: $(find "$work/stock/$k" -type f | wc -l) files"
done

for b in $(seq "$builds"); do
	for k in 1 2; do
		KC_KSP=$ksp KC_STRICT=1 g "$k" -I "$init" || { cat "$log" >&2; exit 1; }
		on_kc 4
		same "$k" "build $b of project $k on kc"
		echo "kc: build $b of project $k: same as stock KSP"
	done
done

mkdir -p "$work/broken"
cp -r "$ksp/." "$work/broken/"
rm "$work/broken/lib/kc-aa.jar"
if KC_KSP=$work/broken KC_STRICT=1 g 1 -I "$init"; then
	echo "gradle.sh: a build without kc-aa.jar passes under KC_STRICT=1: $log" >&2
	exit 1
fi
echo "strict: a failure on kc fails the build"
KC_KSP=$work/broken g 1 -I "$init" || { cat "$log" >&2; exit 1; }
on_kc 0
[ "$(grep -c '^kc: KSP on .* failed, so stock KSP processes it: ' "$log")" = 4 ]
same 1 "the build that fell back to stock KSP"
echo "fallback: stock KSP takes over, same output"

KC_KSP=$ksp KC_STRICT=1 KC_VARIANTS=debug g 1 -I "$init" || { cat "$log" >&2; exit 1; }
on_kc 0
same 1 "the build with KC_VARIANTS=debug"
echo "variants: a module outside KC_VARIANTS keeps stock KSP"

# kept LOADED KEPT: the build made LOADED class loaders of processors and found KEPT kept.
kept() {
	local loaded found
	loaded=$(grep -c '^kc-jvm: processors loaded$' "$log" || true)
	found=$(grep -c '^kc-jvm: processors kept$' "$log" || true)
	[ "$loaded $found" = "$1 $2" ] || { echo "gradle.sh: $loaded class loaders of processors made and $found kept, not $1 and $2: $log" >&2; exit 1; }
}
for k in 1 2; do
	for b in 1 2; do
		KC_KSP=$ksp KC_STRICT=1 KC_KSP_KEEP=1 KC_TIMING=1 g "$k" -I "$init" || { cat "$log" >&2; exit 1; }
		on_kc 4
		[ "$b" = 1 ] || kept 0 4
		same "$k" "build $b of project $k with kept processors"
	done
done
echo "keep: the processors' classes stay loaded, same output"

for k in 1 2; do
	for b in 1 2; do
		KC_KSP=$ksp KC_STRICT=1 KC_KSP_FORGET=1 KC_TIMING=1 g "$k" -I "$init" || { cat "$log" >&2; exit 1; }
		on_kc 4
		forgot=$(grep -c '^kc-jvm: environments forgotten$' "$log" || true)
		[ "$forgot" = 4 ] || { echo "gradle.sh: $forgot runs kept nothing with KC_KSP_FORGET=1, not 4: $log" >&2; exit 1; }
		same "$k" "build $b of project $k with KC_KSP_FORGET=1"
	done
done
echo "forget: the runs keep nothing for the next, same output"

cat > "$work/project1/dagger/src/main/kotlin/Unbound.kt" <<-EOF
	@dagger.Component
	interface Unbound {
	    fun runnable(): Runnable
	}
EOF
for strict in 0 1; do
	if KC_KSP=$ksp KC_STRICT=$strict g 1 -I "$init"; then
		echo "gradle.sh: a build with a processor's error passes: $log" >&2
		exit 1
	fi
	got=$(grep -c 'Dagger/MissingBinding' "$log" || true)
	[ "$got" = 1 ] || { echo "gradle.sh: KC_STRICT=$strict: the processor's error is reported $got times, not once: $log" >&2; exit 1; }
done
echo "errors: a processor's error fails the build, reported once"

mine=$(comm -13 <(sort <<< "$before") <(daemons | sort))
[ "$(wc -w <<< "$mine")" -le 1 ] || { echo "gradle.sh: the builds ran in daemons" $mine >&2; exit 1; }
echo "gradle: ok ($n builds in one daemon)"
