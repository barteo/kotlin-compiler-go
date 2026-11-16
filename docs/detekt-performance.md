# Making detekt on kc faster

This page describes the five changes that most speed up
[detekt on kc](detekt.md), ranked by the time they save, with the
measurements behind each. All five are always on. They were measured one
at a time on the branch `work/detekt-perf-proposals`, where each could be
switched off, and the findings were the same with and without each of
them.

## Summary

| # | Change | kotlinpoet | kotlinx-coroutines-core |
|:---|:---|---:|---:|
| | Baseline, serial | 6.87 s | 11.96 s |
| 1 | Run the rules on all cores (`--parallel`) by default | −1.96 s | −3.40 s |
| 2 | A class-data sharing archive for the JVM | −0.95 s | −1.03 s |
| 3 | Share the `KaType`s and `KaCall`s the session makes | −0.57 s | −0.49 s |
| 4 | Analyze in the background, and decode facts off the rules' thread | −0.36 s | −0.33 s |
| 5 | Stop wasting time in `OpLintAnalyze` (Go) | −0.15 s | −0.11 s |
| | **All five: `run.sh` as it is now** | **3.76 s (−45%)** | **6.95 s (−42%)** |

- Each saving is that change alone against the baseline, as the median
  wall time of 7 runs on kotlinpoet and 5 on coroutines. The savings
  overlap, so they don't add up to the total.
- Change 5 is 25–27% of kc's own analysis time (392 → 295 ms and
  718 → 522 ms). It saves less end to end because that analysis is a small
  part of a detekt run.
- Stock detekt, with the Analysis API's own session, took 23.4 s on
  kotlinpoet on the same machine.

## Setup

- detekt-cli 2.0.0-alpha.6 from Maven Central, `--analysis-mode full
  --all-rules`, on a Linux x86_64 cloud machine with 4 CPUs and 15 GB,
  JDK 21.0.12.
- kotlinpoet 2.4.0, `commonMain` and `jvmMain` (39 files, 8.7k lines), with
  kotlin-stdlib and kotlin-reflect 2.4.21 on the class path.
- kotlinx-coroutines-core 1.11.0, `common`, `concurrent` and `jvm` (169
  files), with atomicfu-jvm 0.32.1 and the two annotation jars of
  [`bench/coroutines-time.sh`](../bench/coroutines-time.sh), and its seven
  `-opt-in`s through `-Dkc.optIns`. detekt-cli has no option for
  `-opt-in`; without them kc rejects the sources and the type-resolution
  rules see no types.
- [`bench/detekt-ab.sh`](../bench/detekt-ab.sh) runs the variants round
  robin and reports the median and minimum wall time and detekt's phase
  timings (`--debug`). It checks every run's findings (file, line, column
  and rule) against the first variant's.
- The baseline is commit 400ff40, before these changes, with the fix
  below. Every other variant turns on one change.

**A fix needed first.** detekt-cli alpha.6 as published crashed on kc:
rules such as `UnusedImport` and `MaxChainedCallsOnSameLine` take a name's
or a call's `mainReference`, and kc's session registered no PSI reference
providers, so `getMainReference` threw `NoSuchElementException`.
[`KcReferences.kt`](../jvm/kc-detekt/src/io/github/barteo/kc/detekt/KcReferences.kt)
now registers references that carry only their element. kc answers
`resolveToSymbol` from the element, so they never resolve through PSI. With
it, kc reports 1580 findings on kotlinpoet, and stock 1597.

## Where the time goes

One baseline run on kotlinpoet under Java Flight Recorder, sampling every
millisecond. JFR samples native frames, where the JVM calls libkc, far more
often than Java frames, so the table splits each 100 ms of the main thread's
wall time evenly over that interval's samples instead of counting them.

| Main thread, one 7.9 s run | Wall | Share |
|:---|---:|---:|
| Loading classes from jars (`URLClassPath`, inflating, defining) | 1.15 s | 14.5% |
| kc's analysis, `OpLintAnalyze`, blocking the first `analyze {}` | 0.48 s | 6.1% |
| Each file's facts: `OpLintFile`, decoding, the PSI join | 0.31 s | 3.9% |
| The rest of the session (`KcAnalysis`, `KcSession`, symbols, types) | 0.54 s | 6.8% |
| The `KaSession` proxy's reflective dispatch | 0.03 s | 0.4% |
| detekt and its rules walking PSI, and the JIT warming up | 5.4 s | 68% |

- All of it is on one thread: without `--parallel`, three of the four
  cores are idle for most of the run.
- detekt's light mode, with no types, spends 2.5–2.6 s in `Analyzer`, and
  kc's full mode 4.9 s. Full mode also runs the type-resolution rules, so
  not all of the difference is kc's.
- The analysis runs from 2.0 s to 2.5 s into the run, half a second after
  kc's session is built at 1.5 s.

The session answers the same questions many times over. These counts are
from the trace counters these changes add (`-Dkc.trace=true`), on
kotlinpoet:

| Question | Calls | Distinct subjects | Repeats |
|:---|---:|---:|---:|
| `KcSession.type`: a kc type to a `KaType` | 286,580 | 2,653 | 99% |
| `KcSession.fact`: an element's fact | 203,746 | 31,094 | 84% |
| `KcAnalysis.call`: an element's `KaCall` | 85,794 | 20,508 | 76% |
| `KcAnalysis.referenced`: the symbol a name refers to | 49,087 | 13,380 | 72% |

On the Go side, `TestLintStages` in
[`internal/lintops/bench_test.go`](../internal/lintops/bench_test.go)
times each stage of the two operations. Before the changes, on
kotlinpoet's 39 files, on 4 cores except the encoding:

| Stage | Time |
|:---|---:|
| Parse | 20–35 ms |
| `analysis.Analyze`: lower, check, build the facts | 332–344 ms |
| `Result.Data`: a deep copy of all 25,396 facts and 3,179 symbols | 60–67 ms |
| Split the facts by file | 5–18 ms |
| Encode every file's facts (1.9 MB), one at a time, as detekt asks | 35–43 ms |

A whole `kc` compile of the same files, code generation included, takes
0.24–0.28 s, so the analysis costs more than compiling. Its CPU profile
shows why: the type checker is 28% of `analysis.Analyze`, and building the
facts afterwards is 63%, on one goroutine.

## 1. Run the rules on all cores by default

**Evidence.**
- The main thread does all the work, and the other three cores are idle
  for most of the run (the profile above).
- With `--parallel`, the baseline goes from 6.87 s to 4.91 s on kotlinpoet
  and from 11.96 s to 8.56 s on coroutines. kc's findings with
  `--parallel` are the same as without it, on both targets.
- Stock detekt gains little from `--parallel`: 49 s against 45.6–60 s on
  coroutines ([detekt on kc](detekt.md)). That is likely because the
  Analysis API's FIR session serializes much of its work. kc's facts are
  computed before the rules run, and the session's lookups go through
  `ConcurrentHashMap`s and per-file locks, so the rules scale.

**Change.** [`run.sh`](../jvm/kc-detekt/run.sh) passes `--parallel`
unless the arguments already have it.

**Result.** −1.96 s on kotlinpoet (−29%) and −3.40 s on coroutines (−28%).
With the other four changes it is 4.58 s and 6.93 s. In parallel, the
other changes save more on coroutines (−1.63 s) than on kotlinpoet
(−0.33 s): there, detekt's own work is what's left.

## 2. A class-data sharing archive

**Evidence.** Loading classes is the largest single cost on the main
thread: 1.15 s, and 60% of the samples of the first 1.25 s, before detekt
has read a file. A run loads 7,437 classes, 5,234 of them from jars.

**Change.** [`run.sh`](../jvm/kc-detekt/run.sh) runs the JVM with
`-XX:SharedArchiveFile=build/kc-detekt-<key>.jsa -XX:+AutoCreateSharedArchive`
(JDK 19 and later). The first run writes the archive (44 MB) when it exits.
Later runs map the classes from it, already parsed and verified.
- The JVM rejects an archive once a jar on its class path changes, as
  `kc-detekt.jar` does with every build, but on JDK 21 it doesn't write a
  new one. Every later run then silently loses the archive.
- So the key is a hash of every class-path jar's path, size and
  modification time. A rebuilt jar gets a new archive, and `run.sh`
  deletes the old ones.

**Result.** −0.95 s on kotlinpoet, with `CreateSettings` going from 1.35 s
to 0.67 s, and −1.03 s on coroutines (1.61 s → 1.03 s). The run that
writes the archive is about 1 s slower than the baseline.

This needs no change to kc, and stock detekt would gain from it too. It
matters here because detekt on kc is a command-line tool that starts a new
JVM every time. On JDK 24 and later, Project Leyden's ahead-of-time cache goes
further: it also links the classes, and from JDK 25 keeps the JIT's
profiles.

## 3. Share the KaTypes and KaCalls the session makes

**Evidence.** The counters above: 99% of `KaType` conversions and 76% of
`KaCall` constructions repeat earlier ones.
- Each conversion parses a `ClassId` from a string, matches a regular
  expression against it and allocates new types.
- A new `KcClassType` starts with an empty `classNames`, the lazily
  computed supertypes that every `isSubtypeOf` reads, so they are computed
  again.
- Each `KaCall` rebuilds its signature, its receivers and a
  `LinkedHashMap` of argument mappings.
- In the profile these are `classTypeOf`, `ClassId.fromString`,
  `Regex.matches` and `functionCall`.

**Change.** `KcSession.type` keeps the `KaType` it made for each kc type,
one for each nullability. `KcAnalysis.call` keeps each element's `KaCall`,
or the absence of one. Both live in `ConcurrentHashMap`s, and both kinds of
object are immutable, so sharing them is safe, also under `--parallel`. A
file's kc types are interned in its facts, so the type caches hold only
2,653 types on kotlinpoet.

**Result.** −0.57 s on kotlinpoet (`Analyzer` 4.87 s → 4.47 s) and −0.49 s
on coroutines (9.68 s → 9.14 s).

**Next.** `fact` and `referenced` repeat nearly as often. A fact is a lookup
already, but through `containingFile`, the file's index and an
`IdentityHashMap`. `referenced` builds string keys with a `StringBuilder`
for library callables each time.

## 4. Analyze in the background, and decode facts off the rules' thread

**Evidence.** kc's analysis runs on the main thread when the first rule
calls `analyze {}`, and every rule waits for it: 0.48 s of 7.9 s. It
starts half a second after the session is built, time the JVM spends on
detekt's setup with three cores idle. Then each file's facts are encoded in
Go and decoded in Java on the main thread, when a rule first asks about
that file: another 0.31 s.

**Change.** `KcSession` starts `OpLintAnalyze` on a background thread as
soon as it is built. kc reads the sources from disk, not detekt's PSI, so
it doesn't have to wait. When the analysis is done, a small pool encodes
and decodes every file's facts, in file order. The main thread only joins a
file's facts with its PSI, when a rule first asks about it, and waits for
that file if it isn't ready yet.

**Result.** −0.36 s on kotlinpoet (`Analyzer` 4.87 s → 4.47 s) and −0.33 s
on coroutines. The analysis is no faster, but it now overlaps detekt's
setup.

**The cost.** detekt builds the session in light mode too, where no rule
asks for types. There the analysis is now wasted: about 0.5 s of CPU on
kotlinpoet, on other cores. `Analyzer` still takes 2.33 s there.

**Next.** The analysis could start earlier still: the source roots are
known when `buildKtModuleProvider` runs, before detekt creates its
`KotlinCoreEnvironment`.

## 5. Stop wasting time in OpLintAnalyze

**Evidence.** The Go stages above. Two of the costs are pure waste:
- `javaFQ`, which turns `a/b/C$D` into `a.b.C.D`, built a new
  `strings.Replacer` on every call. Converting a class type called it up to
  three times (`classFQ`, `classID` and the SAM check). That was 27% of the
  CPU spent building the facts.
- `OpLintAnalyze` took `Result.Data()`, a deep copy of every fact and
  symbol, only to read each of them once while encoding.

**Change.**
- One shared `Replacer`, and a per-class memo in `converter` of the
  converted name, class id and SAM-ness.
- `Result.SharedData()` returns the facts and symbols without copying
  them, for callers that only read them, and `lintops` uses it.

**Result.** Median of 11 rounds, before and after, with the same timing
code:

| | `analysis.Analyze` | Copy, split and encode every file | Total |
|:---|---:|---:|---:|
| kotlinpoet | 290 → 255 ms | 103 → 40 ms | 392 → 295 ms (−25%) |
| kotlinx-coroutines-core | 518 → 448 ms | 199 → 74 ms | 718 → 522 ms (−27%) |

End to end, without the background analysis, that is −0.15 s on kotlinpoet
and −0.11 s on coroutines. With change 4, more of what's left is hidden
behind detekt's setup.

**Next.** Building the facts (`builder.build`) is still serial and the
largest part of the analysis. Each file's facts depend only on that file's
AST and the shared symbol table, so it could run per file on all cores, as
checking does, with symbol IDs assigned in a final serial pass. Encoding
every file's facts on 4 goroutines instead of one already measures
35–43 ms → 16–22 ms.

## What doesn't help

- **C1 only** (`-XX:TieredStopAtLevel=1`) is often good for short JVM runs,
  but here it is slower: `Analyzer` takes 6.0–7.6 s with the archive,
  against 4.5–4.7 s. The rules run long enough for C2 to pay off.
- **Replacing the `KaSession` proxy** with generated code isn't worth it.
  Its reflective dispatch is 0.03 s of the run, for about 300,000 calls.

## Reproducing

The comparison of each change alone needs `work/detekt-perf-proposals`,
where `-Dkc.eager=false` and `-Dkc.cache=false` turn off changes 4 and 3,
and the archive and `--parallel` are JVM and detekt options. On it:

```
DETEKT_HOME=.../detekt-cli jvm/kc-detekt/build.sh
cp jvm/kc-detekt/build/libkc.so /tmp/libkc-new.so
# A libkc.so of the commit before the changes, for the baseline:
git worktree add /tmp/base 400ff40 && (cd /tmp/base &&
	CGO_CFLAGS="-I$JAVA_HOME/include -I$JAVA_HOME/include/linux" \
	go build -tags libkc -buildmode=c-shared -o /tmp/libkc-base.so ./cmd/libkc)
ARGS="-i src/commonMain -i src/jvmMain --analysis-mode full --all-rules -cp ... --jdk-home $JAVA_HOME"
RUNS=7 bench/detekt-ab.sh /tmp/ab \
	"base|/tmp/libkc-base.so|-Dkc.eager=false -Dkc.cache=false" \
	"go|/tmp/libkc-new.so|-Dkc.eager=false -Dkc.cache=false" \
	"eager|/tmp/libkc-base.so|-Dkc.cache=false" \
	"cache|/tmp/libkc-base.so|-Dkc.eager=false" \
	"cds|/tmp/libkc-base.so|-Dkc.eager=false -Dkc.cache=false -XX:SharedArchiveFile=/tmp/cds.jsa -XX:+AutoCreateSharedArchive" \
	"all|/tmp/libkc-new.so|-XX:SharedArchiveFile=/tmp/all.jsa -XX:+AutoCreateSharedArchive" \
	-- $ARGS
```

- Add `--parallel` after `$ARGS` for change 1.
- On this branch, compare whole builds instead: a variant can name its own
  `kc-detekt.jar`, such as one built at 400ff40 with the reference fix, as
  `"base|/tmp/base/libkc.so||/tmp/base/kc-detekt.jar"`.
- `KC_LINT_SRC=src/commonMain:src/jvmMain KC_LINT_CP=... go test ./internal/lintops -run TestLintStages -v`
  times the Go stages.
- `KC_JAVA_OPTS=-Dkc.trace=true jvm/kc-detekt/run.sh ...` prints the
  repeat counts.
- `KC_JAVA_OPTS="-XX:StartFlightRecording=filename=kc.jfr,settings=profile,jdk.ExecutionSample#period=1ms"`
  records the profile.

The raw results of the runs above:

```
kotlinpoet, 7 runs, serial
variant        median      min   CreateSettings   Analyzer findings
base            6.87s    6.51s            1.35s      4.87s yes (1580)
go              6.72s    6.18s            1.24s      4.69s yes (1580)
eager           6.51s    6.07s            1.28s      4.47s yes (1580)
cache           6.30s    5.92s            1.24s      4.47s yes (1580)
cds             5.92s    5.48s            0.67s      4.78s yes (1580)
all             5.37s    5.07s            0.65s      4.24s yes (1580)

kotlinx-coroutines-core, 5 runs, serial
base           11.96s   11.49s            1.61s      9.68s yes (2773)
go             11.85s   11.18s            1.65s      9.49s yes (2773)
eager          11.63s   11.30s            1.62s      9.27s yes (2773)
cache          11.47s   11.29s            1.57s      9.14s yes (2773)
cds            10.93s   10.23s            1.03s      9.44s yes (2773)
all            10.26s    9.90s            1.08s      8.63s yes (2773)

--parallel, 5 runs (findings the same as serial)
kotlinpoet  base   4.91s  4.78s  1.17s  3.10s
kotlinpoet  all    4.58s  4.21s  1.22s  2.64s
coroutines  base   8.56s  7.99s  1.56s  6.14s
coroutines  all    6.93s  6.52s  1.00s  5.33s

run.sh with its defaults (all five), 5 runs, after the run that writes the archive
kotlinpoet  3.56 3.58 3.76 4.05 4.36 s   (findings the same as the baseline's)
coroutines  6.86 6.93 6.95 7.01 7.09 s   (findings the same as the baseline's)
```
