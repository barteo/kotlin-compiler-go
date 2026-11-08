# workload.sh has what the processor workloads' run.sh scripts
# (bench/ksp/room, bench/ksp/hilt) share; they source it after setting
# root. It sets kcaa, out and lib and defines:
#
#	paths GROUP:ARTIFACT:VERSION...
#		prints the jars jvm/kc-aa/build.sh fetched, one per line
#	same_output SRC LIBS_FILE PROCS_FILE WORK [STOCK_EXTRA]
#		runs the processors once on each backend (the build.sh test's
#		RunKsp) into WORK/kc and WORK/stock, and fails unless both
#		generate the same files
kcaa=$root/jvm/kc-aa
out=$kcaa/build
lib=$root/jvm/ksp/build/aa/m2
export JAVA_TOOL_OPTIONS=
java_home=${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")}

paths() {
	local c g a v
	for c; do
		IFS=: read -r g a v <<< "$c"
		[ -s "$out/m2/$a-$v.jar" ] || { echo "run.sh: no $a-$v.jar: run jvm/kc-aa/build.sh stock first" >&2; exit 1; }
		echo "$out/m2/$a-$v.jar"
	done
}

same_output() {
	local src=$1 libs procs work=$4 extra=${5:-} kspwork=$root/jvm/ksp/build runtime deps stockcp= jar
	libs=$(paste -sd: "$2")
	procs=$(paste -sd: "$3")
	runtime=$lib/kotlin-compiler-2.4.21.jar:$lib/kotlin-stdlib-2.4.21.jar:$lib/kotlin-reflect-2.4.21.jar:$lib/kotlinx-coroutines-core-jvm-1.8.0.jar:$lib/annotations-23.0.0.jar
	deps=$kspwork/aa/kc-aa-api-2.4.21.jar:$runtime:$lib/caffeine-2.9.3.jar:$kspwork/ksp-aa-kc.jar
	deps=$deps:$kspwork/m2/symbol-processing-api-2.3.9.jar:$kspwork/m2/symbol-processing-common-deps-2.3.9.jar
	deps=$deps:$kspwork/m2/kotlinx-serialization-core-jvm-1.6.3.jar:$kspwork/m2/kotlinx-serialization-json-jvm-1.6.3.jar
	for jar in symbol-processing-aa-embeddable-2.3.6 symbol-processing-api-2.3.6 symbol-processing-common-deps-2.3.6 \
		kotlin-stdlib-2.3.0 kotlinx-coroutines-core-jvm-1.8.0-intellij-14; do
		stockcp=$stockcp:$out/m2/$jar.jar
	done
	[ -d "$out/runksp-classes" ] && [ -d "$out/runksp-stock" ] || { echo "run.sh: run jvm/kc-aa/build.sh stock first" >&2; exit 1; }
	rm -rf "$work/kc" "$work/stock"
	LD_PRELOAD=$java_home/lib/libjsig.so java -Dksp.backend=kc -Dkc.lib="$out/libkc.so" -Didea.home.path="$out" \
		-cp "$out/runksp-classes:$procs:$out/kc-aa.jar:$deps" RunKspKt "$src" "$work/kc" "$libs" > "$work/kc.txt"
	java -Dksp.languageVersion=2.3 -cp "$out/runksp-stock:$procs${extra:+:$extra}$stockcp" RunKspKt "$src" "$work/stock" "$libs" > "$work/stock.txt"
	rm -rf "$work"/{kc,stock}/{caches,classes}
	diff -ru "$work/stock" "$work/kc"
	echo "kc generates what stock KSP does ($(find "$work/kc" -type f | wc -l) files)"
}
