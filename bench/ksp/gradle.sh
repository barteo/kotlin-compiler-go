#!/usr/bin/env bash
# gradle.sh times KSP on kc in a real Gradle build: the KSP Gradle plugin
# 2.3.13 and the Kotlin Gradle plugin 2.4.21, over a generated project of
# MODULES modules, each with the sources and processors of PROCESSORS:
# Moshi models and Moshi's codegen, and with "dagger", a Dagger graph and
# Dagger's processor. kc takes the place of KSP's Analysis API: a local Maven
# repository serves symbol-processing-aa-embeddable 2.3.13 as a jar whose
# manifest puts the KSP fork, kc-aa and their dependencies on the class
# path, and symbol-processing-api 2.3.13 as the fork's.
#
#	bench/ksp/gradle.sh [BUILDS]
#
# For each setting of the plugin's processor class-loader cache
# (ksp.classloader.cache.processors), it starts a daemon and runs BUILDS
# builds (default 4) of every module's kspKotlin in it (--rerun, so each
# does a full KSP run), and prints each build's wall time and the sum of
# its kspKotlin tasks' times. With PROJECTS > 1 the builds go to that many
# copies of the project in turn, in the same daemon, each with a KSP class
# path of its own (as with projects on different KSP or kc builds), so the
# plugin loads KSP and kc-aa in a class loader per project. It needs
# jvm/kc-aa/build.sh to have run.
# Environment:
#	MODULES   modules in the project (default 6)
#	PROJECTS  copies of the project (default 1)
#	PROCESSORS  "moshi", "dagger" or both (default moshi). Dagger fails on
#	          kc in the plugin: its processor class loader's parent, KSP's,
#	          has kotlin-compiler's unshaded Guava, which splits
#	          com.google.common.graph with Dagger's own (IllegalAccessError).
#	CACHE     the cache settings to time (default "false true")
#	KC_AA     the kc-aa jar (default jvm/kc-aa/build/kc-aa.jar)
#	PARALLEL  org.gradle.parallel (default true)
#	CONFIG_CACHE  org.gradle.configuration-cache (default true). With it,
#	          Gradle makes the plugin's class-loader cache service, which
#	          closes KSP's class loaders at the end of each build, so each
#	          build loads KSP and kc-aa anew; without it, the service is
#	          never made and the class loaders stay. The kspKotlin tasks'
#	          time is printed only without it.
#	JVM_OPTS  more options for the daemon's JVM
#	GRADLE    the gradle command (default gradle)
set -euo pipefail

builds=${1:-4}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
kcaa=$root/jvm/kc-aa
out=$kcaa/build
work=$out/gradle-bench
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}
export JAVA_TOOL_OPTIONS=${KC_JAVA_TOOL_OPTIONS:-}
kc_aa=$(readlink -f "${KC_AA:-$out/kc-aa.jar}")
[ -s "$kc_aa" ] && [ -s "$out/libkc.so" ] || { echo "gradle.sh: run jvm/kc-aa/build.sh first" >&2; exit 1; }

kspwork=$root/jvm/ksp/build
lib=$kspwork/aa/m2
m2=$kspwork/m2
modules=${MODULES:-6}

# The local Maven repositories, one per project (repoK): their
# aa-embeddable jars differ only in a manifest attribute naming K, so each
# project's KSP class path, and so its KSP class loader, is its own.
projects=${PROJECTS:-1}
rm -rf "$work"/repo* "$work/thin"
put() { # put REPO ARTIFACT JAR: com.google.devtools.ksp:ARTIFACT:2.3.13
	local d=$1/com/google/devtools/ksp/$2/2.3.13
	mkdir -p "$d"
	cp "$3" "$d/$2-2.3.13.jar"
	cat > "$d/$2-2.3.13.pom" <<-EOF
	<project><modelVersion>4.0.0</modelVersion><groupId>com.google.devtools.ksp</groupId>
	<artifactId>$2</artifactId><version>2.3.13</version></project>
	EOF
}
cp=
for j in "$kc_aa" "$kspwork/ksp-aa-kc.jar" "$kspwork/aa/kc-aa-api-2.4.21.jar" "$lib/kotlin-compiler-2.4.21.jar" \
	"$lib/kotlin-reflect-2.4.21.jar" "$lib/kotlinx-coroutines-core-jvm-1.8.0.jar" "$lib/annotations-23.0.0.jar" \
	"$lib/caffeine-2.9.3.jar" "$m2/kotlinx-serialization-core-jvm-1.6.3.jar" "$m2/kotlinx-serialization-json-jvm-1.6.3.jar"; do
	cp="$cp file:$j"
done
for k in $(seq "$projects"); do
	mkdir -p "$work/thin/$k"
	# A manifest line holds at most 72 bytes; longer ones continue on lines
	# that start with a space.
	{
		echo "Kc-Bench-Project: $k"
		printf 'Class-Path:%s' "$cp" | awk '{ print substr($0, 1, 72); for (i = 73; i <= length($0); i += 71) print " " substr($0, i, 71) }'
	} > "$work/thin/$k/manifest.txt"
	(cd "$work/thin/$k" && jar cfm thin.jar manifest.txt)
	put "$work/repo$k" symbol-processing-aa-embeddable "$work/thin/$k/thin.jar"
	put "$work/repo$k" symbol-processing-api "$kspwork/symbol-processing-api.jar"
done

# The project.
rm -rf "$work"/project*
proj=$work/project1
mkdir -p "$proj"
{
	echo 'pluginManagement { repositories { mavenCentral(); gradlePluginPortal() } }'
	echo 'dependencyResolutionManagement {'
	echo '    repositories {'
	echo "        exclusiveContent { forRepository { maven { url = uri(\"file://$work/repo1\") } }"
	echo '            filter { includeModule("com.google.devtools.ksp", "symbol-processing-aa-embeddable")'
	echo '                     includeVersion("com.google.devtools.ksp", "symbol-processing-api", "2.3.13") } }'
	echo '        mavenCentral()'
	echo '    }'
	echo '}'
	echo 'rootProject.name = "kc-gradle-bench"'
	for i in $(seq "$modules"); do echo "include(\":m$i\")"; done
} > "$proj/settings.gradle.kts"
cat > "$proj/build.gradle.kts" <<-'EOF'
plugins {
    kotlin("jvm") version "2.4.21" apply false
    id("com.google.devtools.ksp") version "2.3.13" apply false
}
EOF
for i in $(seq "$modules"); do
	m=$proj/m$i
	mkdir -p "$m/src/main/kotlin/m$i" "$m/src/main/kotlin/d$i"
	{
		echo 'plugins {'
		echo '    kotlin("jvm")'
		echo '    id("com.google.devtools.ksp")'
		echo '}'
		echo 'dependencies {'
		for p in ${PROCESSORS:-moshi}; do
			case $p in
			moshi)
				echo '    implementation("com.squareup.moshi:moshi:1.15.2")'
				echo '    ksp("com.squareup.moshi:moshi-kotlin-codegen:1.15.2")'
				sed "s/^package m$/package m$i/" "$kcaa/test/moshi/src/m/Models.kt" > "$m/src/main/kotlin/m$i/Models.kt"
				;;
			dagger)
				echo '    implementation("com.google.dagger:dagger:2.59.2")'
				echo '    ksp("com.google.dagger:dagger-compiler:2.59.2")'
				sed "s/^package d$/package d$i/" "$kcaa/test/dagger/src/d/App.kt" > "$m/src/main/kotlin/d$i/App.kt"
				;;
			*) echo "gradle.sh: unknown processor $p" >&2; exit 2 ;;
			esac
		done
		echo '}'
	} > "$m/build.gradle.kts"
done
cat > "$work/timing.init.gradle" <<-'EOF'
	def starts = [:].asSynchronized()
	def total = new java.util.concurrent.atomic.AtomicLong()
	gradle.addListener(new TaskExecutionListener() {
	    void beforeExecute(Task t) { starts[t.path] = System.nanoTime() }
	    void afterExecute(Task t, TaskState s) {
	        if (t.name == 'kspKotlin') total.addAndGet(System.nanoTime() - starts[t.path])
	    }
	})
	gradle.buildFinished { println "kspKotlin tasks: ${(long) (total.get() / 1e6)} ms" }
	EOF

for k in $(seq 2 "$projects"); do
	cp -r "$proj" "$work/project$k"
	sed -i -e "s/^rootProject.name = .*/rootProject.name = \"kc-gradle-bench-$k\"/" \
		-e "s|file://$work/repo1|file://$work/repo$k|" "$work/project$k/settings.gradle.kts"
done
g() { # g K ARGS... builds project K
	local k=$1
	shift
	local timing=(-I "$work/timing.init.gradle")
	[ "${CONFIG_CACHE:-true}" = true ] && timing=()
	LD_PRELOAD=$java_home/lib/libjsig.so ${GRADLE:-gradle} -q --daemon -p "$work/project$k" \
		--project-cache-dir "$work/gradle-cache-$k" "${timing[@]}" "$@"
}
props() {
	for k in $(seq "$projects"); do
	cat > "$work/project$k/gradle.properties" <<-EOF
	org.gradle.jvmargs=-Xmx6g -Xss8m -Dksp.backend=kc -Dkc.lib=$out/libkc.so -Didea.home.path=$out ${JVM_OPTS:-}
	org.gradle.parallel=${PARALLEL:-true}
	org.gradle.configuration-cache=${CONFIG_CACHE:-true}
	ksp.classloader.cache.processors=$1
	kotlin.compiler.execution.strategy=in-process
	EOF
	done
}
# A first build in a daemon of its own fetches the jars, retrying on the
# 429s Maven Central sends to busy clients.
props false
for i in 1 2 3 4 5 6; do
	g 1 kspKotlin > "$work/fetch.log" 2>&1 && break
	grep -q 'status code 429' "$work/fetch.log" || { cat "$work/fetch.log"; exit 1; }
	sleep $((i * 10))
done
for cache in ${CACHE:-false true}; do
	props "$cache"
	${GRADLE:-gradle} -q --stop -p "$proj" >/dev/null
	echo "== ksp.classloader.cache.processors=$cache, configuration cache ${CONFIG_CACHE:-true}, $modules modules, $projects project(s), ${PROCESSORS:-moshi}, kc-aa $kc_aa"
	for b in $(seq "$builds"); do
		k=$(( (b - 1) % projects + 1 ))
		t0=$(date +%s%N)
		if g "$k" kspKotlin --rerun > "$work/build-$cache-$b.log" 2>&1; then
			echo "build $b (project $k): $(( ($(date +%s%N) - t0) / 1000000 )) ms wall, $(grep -h '^kspKotlin tasks:' "$work/build-$cache-$b.log" || true)"
		else
			echo "build $b (project $k): FAILED: $(grep -m1 -h -o 'UnsatisfiedLinkError.*\|NoClassDefFoundError.*\|Caused by: .*' "$work/build-$cache-$b.log" || tail -3 "$work/build-$cache-$b.log")"
		fi
	done
	${GRADLE:-gradle} -q --stop -p "$proj" >/dev/null
done
