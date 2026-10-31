#!/bin/sh
# Surveys the Kotlin JVM libraries that bench/survey/repos.txt lists, each
# at its pinned release tag, for the features kc doesn't support, and
# ranks them by the files they block across all of them. Writes
# bench/results/survey.md, and each repository's report to
# bench/results/survey/.
#
#   bench/survey.sh       # about 2 s per repository after the first clone
#
# Needs: go, git and network access to github.com for the first run, the
# Kotlin standard library (KOTLIN_HOME, as for bench/run.sh), whose
# metadata lists the inline-only functions, and a JDK (JAVA_HOME) for the
# checker's cross-check.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
export JAVA_HOME=${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}
: "${TMPDIR:=/tmp}"
cache="$TMPDIR/kc-survey"
manifest="$root/bench/survey/repos.txt"
kotlin_home=${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}
stdlib="$kotlin_home/lib/kotlin-stdlib.jar"
cp="$stdlib"
if [ -f "$kotlin_home/lib/kotlin-reflect.jar" ]; then
	cp="$cp:$kotlin_home/lib/kotlin-reflect.jar"
fi

mkdir -p "$cache"
grep -v '^#' "$manifest" | while read -r name url tag _; do
	[ -n "$name" ] || continue
	if [ ! -d "$cache/$name-$tag" ]; then
		echo "cloning $name $tag"
		git -c advice.detachedHead=false clone -q --depth 1 --branch "$tag" "$url" "$cache/$name-$tag"
	fi
done
cd "$root"
go build -o "$cache/survey" ./bench/survey
"$cache/survey" -repos "$manifest" -cache "$cache" -stdlib "$stdlib" -cp "$cp" \
	-o bench/results/survey.md -dir bench/results/survey
echo "wrote bench/results/survey.md and bench/results/survey/"
