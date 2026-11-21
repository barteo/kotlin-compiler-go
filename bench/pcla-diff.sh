#!/bin/sh
# Compares what kc compiles with two settings of $KC_PCLA, the builder
# inference mode (internal/check/pcla.go): the class files and the
# diagnostics of testdata/run, testdata/errors and the conformance
# tests. Every difference is a decision that one mode makes from a type
# the other doesn't have yet.
#
#	bench/pcla-diff.sh [-a recheck] [-b single] [-o dir] [-only testdata|box|diagnostics] [harness flags]
#
# The harness flags, as -filter tests/inference/pcla/, go to both
# conformance runs. The outputs stay in dir, $TMPDIR/kc-pcla-diff by
# default, as dir/a and dir/b; the differences are listed on stdout and
# the script fails if there are any. With -a and -b the same, it checks
# that kc's output is deterministic.
set -e
a=recheck b=single only=
out="${TMPDIR:-/tmp}/kc-pcla-diff"
while [ $# -gt 0 ]; do
	case "$1" in
	-a) a=$2; shift 2 ;;
	-b) b=$2; shift 2 ;;
	-o) out=$2; shift 2 ;;
	-only) only=$2; shift 2 ;;
	*) break ;;
	esac
done
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
kotlin_home="${KOTLIN_HOME:-/opt/homebrew/opt/kotlin/libexec}"
lib="$kotlin_home/lib"
cp="$lib/kotlin-stdlib.jar:$lib/kotlin-reflect.jar:$lib/kotlin-test.jar:$(ls "$lib"/kotlinx-coroutines-core*.jar | head -1)"
rm -rf "$out"
mkdir -p "$out"
go build -o "$out/kc" ./cmd/kc

# one compiles a program of testdata, $1, into directory $2.
cat > "$out/one.sh" <<'EOS'
#!/bin/sh
src=$1 dst=$2/$(basename "$1" .kt)
mkdir -p "$dst"
main=$src
[ -d "$src" ] && main=$(ls "$src"/*.kt | head -1)
flags=$(sed -n '1s|^// kotlinc: ||p' "$main")
"$KC" -d "$dst" -cp "$CP" -j 1 $flags "$src" > "$dst/compiler.out" 2>&1 || true
EOS
chmod +x "$out/one.sh"

run() { # mode dir
	if [ -z "$only" ] || [ "$only" = testdata ]; then
		for kind in run errors; do
			mkdir -p "$2/$kind"
			ls -d testdata/$kind/*.kt testdata/$kind/*/ | sed 's|/$||' |
				KC_PCLA=$1 KC="$out/kc" CP="$cp" xargs -P "$(getconf _NPROCESSORS_ONLN)" -I{} "$out/one.sh" {} "$2/$kind"
		done
	fi
	shift_mode=$1 dir=$2
	shift 2
	for mode in box diagnostics; do
		if [ -z "$only" ] || [ "$only" = "$mode" ]; then
			KC_PCLA=$shift_mode bench/kotlin-tests.sh $mode -o "" -tsv "" -missed "" -keep -work "$dir/$mode" "$@" > "$dir/$mode.log" 2>&1 || true
			tail -n +1 "$dir/$mode.log" | grep -v '^[0-9]*/[0-9]*$' | sed "s|^|$shift_mode $mode: |"
		fi
	done
}
mkdir -p "$out/a" "$out/b"
run "$a" "$out/a" "$@"
run "$b" "$out/b" "$@"
if diff -r -q -x '*.log' "$out/a" "$out/b" > "$out/diff.txt"; then
	echo "no differences between KC_PCLA=$a and KC_PCLA=$b"
	exit 0
fi
# The differing tests, and the diagnostics that differ.
sed -n 's|^Files \(.*\) and .* differ$|\1|p; s|^Only in \(.*\): \(.*\)$|\1/\2|p' "$out/diff.txt" | while read -r f; do
	rel=${f#"$out/a/"}
	rel=${rel#"$out/b/"}
	case "$rel" in
	*compiler.out)
		echo "== $rel"
		diff "$out/a/$rel" "$out/b/$rel" || true
		;;
	*) echo "differs: $rel$(sed -n '1s|^test: | (|p' "$out/a/${rel%%/out/*}/compiler.out" 2>/dev/null | sed 's|$|)|')" ;;
	esac
done
exit 1
