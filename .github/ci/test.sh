#!/usr/bin/env bash
# Runs all tests the way CI does, and fails if any test fails or is
# skipped for a reason other than the expected ones below. Arguments are
# passed to go test, for example:
#
#   .github/ci/test.sh -race
#
# The tests need a JDK, kotlinc and the jars that .github/ci/deps.sh
# downloads; run it first, or see "Tests" in README.md.
#
# The driver package runs in four processes rather than one. A test
# process keeps the classes of every compilation it has done, so
# internal/driver's tests in one process peak at about 8 GB, and at more
# than 13 GB under -race, more than CI runners have. Split, the largest
# part peaks at about 2 GB, and 8 GB under -race. The four parts cover
# every test: TestRun's programs without the standard library, its
# stdlib* programs in two halves, and the other tests.
set -uo pipefail
cd "$(dirname "$0")/../.."

status=0
gotest() { go test -json "$@" || status=1; }
{
	gotest "$@" $(go list ./... | grep -v '/internal/driver$')
	gotest "$@" ./internal/driver -skip '^TestRun$'
	gotest "$@" ./internal/driver -run '^TestRun$' -skip '^TestRun$/^stdlib'
	gotest "$@" ./internal/driver -run '^TestRun$/^stdlib[A-K]'
	gotest "$@" ./internal/driver -run '^TestRun$/^stdlib' -skip '^TestRun$/^stdlib[A-K]'
	exit $status
} | go run .github/ci/testcheck/main.go \
	-allow '^needs the standard library$' \
	-allow '^set KMETA_DUMP to class files to dump$'
