#!/usr/bin/env bash
# fetch.sh gets the inputs of KSP's own benchmark, TachiyomiExhaustive
# (github.com/google/ksp, benchmark/), for bench/ksp/run.sh.
#
#	bench/ksp/exhaustive/fetch.sh [DIR]
#
# Upstream's benchmark/TachiyomiExhaustive/build.sh clones
# inorichi/tachiyomi at 938339690e, applies tachi.patch (kept here) and runs
# its copyDepsDevDebug task to dump the app's compile class path into lib/.
# That repository is gone and the build needs the Android SDK, so this
# fetches the bundle upstream made of the two (upload-benchmark-data.sh),
# which KSP's benchmark CI downloads: the patched checkout and lib/, 136
# jars. Its lib/kotlin-compiler.jar is 1.8.0-dev-1390, tachi.patch's.
#
# It writes into DIR (default bench/ksp/exhaustive/data):
#	src/       the checkout's .kt files, 559 of them, at their paths: what
#	           upstream's runner.sh passes (find -name '*.kt')
#	libs.txt   the class path, lib/*.jar, one per line
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
dir=$(mkdir -p "${1:-$here/data}" && cd "${1:-$here/data}" && pwd)
sha=465107197b05d05b254751dc976ced2492761786b4fde3c3187211707f08922b
url=https://storage.googleapis.com/r8-deps/ksp-bench/$sha.tar.gz

if [ ! -s "$dir/libs.txt" ]; then
	tgz=$dir/bundle.tar.gz
	if [ ! -s "$tgz" ]; then
		curl -sSfL -o "$tgz.tmp" "$url"
		mv "$tgz.tmp" "$tgz"
	fi
	echo "$sha  $tgz" | sha256sum -c --quiet
	rm -rf "$dir/bundle" "$dir/src"
	mkdir "$dir/bundle" "$dir/src"
	tar xzf "$tgz" -C "$dir/bundle" --no-same-owner
	(cd "$dir/bundle/tachiyomi" && find . -name '*.kt' -exec cp --parents -p -t "$dir/src/" {} +)
	ls "$dir"/bundle/lib/*.jar >"$dir/libs.txt"
	rm "$tgz"
fi
echo "fetch.sh: $(find "$dir/src" -name '*.kt' | wc -l) files in $dir/src, $(wc -l <"$dir/libs.txt") jars in $dir/libs.txt"
