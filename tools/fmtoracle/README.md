# fmtoracle

`fmtoracle.sh` gives `TestFormatBudget` (internal/ide) IntelliJ's
formatting of the corpus as its reference. It runs IntelliJ IDEA
Community's command-line formatter, in the Kotlin official style, over a
copy of every `.kt` file under `testdata/`. CI never runs it, so the Go
tests need neither IntelliJ nor a JVM.

## Run

Install IntelliJ IDEA Community (2024.1 or newer, whose bundled Kotlin
plugin has the official style as its default), then, from the
repository root:

```bash
tools/fmtoracle/fmtoracle.sh /path/to/idea
```

No IDE instance may be running with the same configuration directory:
`format.sh` starts IntelliJ headless.

It writes:

- `internal/ide/testdata/format.sum`: the sha256 of IntelliJ's output per
  corpus file, relative to `testdata/`. When this file exists,
  `TestFormatBudget` compares `ide.Format`'s output with it instead of with
  the corpus as written.
- `internal/ide/testdata/format-intellij.diff`: a unified diff of
  IntelliJ's output (`intellij/`) against `ide.Format`'s (`kc/`), for
  review.

Then set `internal/ide/testdata/format-budget.txt` to the files the test
reports as different, and commit all three.

## Status

Not run yet: the environment the formatter was written in has no
IntelliJ. Until it is, the budget's reference is the corpus as written,
formatting which reproduces the file. docs/lsp.md lists this as a gap.
