#!/usr/bin/env bash
# fmtoracle.sh IDEA_HOME
#
# Formats a copy of the corpus (testdata/**/*.kt) with IntelliJ IDEA's
# command-line formatter in the Kotlin official style, and writes
#   internal/ide/testdata/format.sum      sha256 of IntelliJ's output per file
#   internal/ide/testdata/format-intellij.diff
#                                         kc's output against IntelliJ's
# Run it from the repository root, by hand; CI never does.
set -euo pipefail
idea=${1:?usage: fmtoracle.sh IDEA_HOME}
root=$(pwd)
[ -f go.mod ] && [ -d testdata ] || { echo "run from the repository root" >&2; exit 1; }
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/intellij" "$work/kc"
(cd testdata && find . -name '*.kt' -print0 | cpio -0 -pdm --quiet "$work/intellij")

# The Kotlin official style is IntelliJ's default for Kotlin; the
# settings file only fixes the indent at 4 spaces.
"$idea/bin/format.sh" -s "$root/tools/fmtoracle/kotlin-official.xml" -r -m '*.kt' "$work/intellij"

(cd "$work/intellij" && find . -name '*.kt' | LC_ALL=C sort | sed 's#^\./##' | xargs sha256sum) \
	> internal/ide/testdata/format.sum

KC_FORMAT_OUT="$work/kc" go test ./internal/ide -run '^TestFormatBudget$' -count=1 || true
(cd "$work" && diff -ru intellij kc) > internal/ide/testdata/format-intellij.diff || true
echo "wrote internal/ide/testdata/format.sum and format-intellij.diff"
echo "now update internal/ide/testdata/format-budget.txt to the files TestFormatBudget reports"
