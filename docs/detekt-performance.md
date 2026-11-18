# Making detekt on kc faster

This page describes the changes that speed up [detekt on kc](detekt.md),
with the measurements behind each, in two rounds. The first five took
detekt on kc to 3.8× and 3.3× stock detekt with `--parallel`.
[A second round](#a-second-round-toward-5-stock---parallel) aimed at 5×. It
reached 4.7× on kotlinpoet and 4.2× on kotlinx-coroutines-core, measured on
a machine where stock ran relatively faster than in the first round.

The first five are ranked by the time they save. All five are always on. They were measured one
at a time on the branch `work/detekt-perf-proposals`, where each could be
switched off, and the findings were the same with and without each of
them.

They were measured on `jvm/kc-detekt`, the first detekt on kc: the
released detekt-cli with its own Analysis API session (`KcSession`) and
lint operations (`internal/lintops`, `OpLintAnalyze` and `OpLintFile`).
detekt on kc now runs on `jvm/kc-aa`, kc's Analysis API that KSP uses too,
and its facts come from `analysis/ka`'s check (`OpKaOpen`, `OpKaFacts`).
All five changes carry over, and [On kc-aa](#on-kc-aa) measures them
there. The sections below keep the names of the code they were made in.

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
`KcReferences.kt` (now
[in kc-aa](../jvm/kc-aa/src/io/github/barteo/kc/aa/KcReferences.kt))
registers references that carry only their element. kc answers
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

On the Go side, `TestLintStages` in `internal/lintops/bench_test.go` (now
[`internal/kaops/bench_test.go`](../internal/kaops/bench_test.go)) times
each stage of the two operations. Before the changes, on
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

**Change.** [`run.sh`](../jvm/detekt/run.sh) passes `--parallel`
unless the arguments already have it.

**Result.** −1.96 s on kotlinpoet (−29%) and −3.40 s on coroutines (−28%).
With the other four changes it is 4.58 s and 6.93 s. In parallel, the
other changes save more on coroutines (−1.63 s) than on kotlinpoet
(−0.33 s): there, detekt's own work is what's left.

## 2. A class-data sharing archive

**Evidence.** Loading classes is the largest single cost on the main
thread: 1.15 s, and 60% of the samples of the first 1.25 s, before detekt
has read a file. A run loads 7,437 classes, 5,234 of them from jars.

**Change.** [`run.sh`](../jvm/detekt/run.sh) runs the JVM with
`-XX:SharedArchiveFile=build/cds/detekt-<key>.jsa -XX:+AutoCreateSharedArchive`
(JDK 19 and later). The first run writes the archive (44 MB) when it exits.
Later runs map the classes from it, already parsed and verified.
- The JVM rejects an archive once a jar on its class path changes, as
  `kc-detekt.jar` (now `kc-aa.jar`) does with every build, but on JDK 21 it doesn't write a
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
`KotlinCoreEnvironment`. On kc-aa, detekt's patched setup opens kc's
session (`KcStandalone.open`) and the analysis starts there.

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
  them, for callers that only read them, and `lintops` uses it (now
  `kaops`, for `OpKaFacts`).

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

- **C1 only** (`-XX:TieredStopAtLevel=1`) is slower serially: `Analyzer`
  takes 6.0–7.6 s with the archive, against 4.5–4.7 s. With `--parallel`
  it is faster, but `run.sh` doesn't tune the JVM's compilers (see
  [What isn't used](#what-isnt-used)).
- **Replacing the `KaSession` proxy** with generated code saves little at
  run time: its reflective dispatch is 0.03 s of the run, for about 300,000
  calls. It does cost the JIT about 0.8 s of CPU (see the
  [second round](#next)).

## On kc-aa

detekt on kc-aa is detekt-cli built from its sources with
[three patches](../jvm/detekt/patches), on `KcStandalone`'s session. The
five changes are ported:

| # | On kc-detekt | On kc-aa |
|:---|:---|:---|
| 1 | `run.sh` adds `--parallel` | the same, in [`jvm/detekt/run.sh`](../jvm/detekt/run.sh) |
| 2 | a CDS archive named by the class path's jars | the same, in `jvm/detekt/build/cds` |
| 3 | `KcSession.type`, `KcAnalysis.call` keep their results | `KcLint.type`, `KcLintAnalysis.call` |
| 4 | `KcSession` analyzes in the background; a pool decodes the facts | `KcStandalone.Session` does, from when it is opened |
| 5 | `Result.SharedData` and the `Replacer` memo, for `OpLintAnalyze` | the same, for `OpKaFacts` |

Two more things matter on kc-aa, where kc's analysis also builds the
model KSP uses, and where nothing parsed the PSI before the first rule:
- **kc checks from the paths.** kc's check needs only the source files'
  paths, so it no longer waits for the session's `KtFile`s, which took
  0.5–0.6 s to make on kotlinpoet.
- **The PSI is parsed on the pool.** While kc checks, the pool parses every
  file's tree, which the round's index of the PSI needs. Building the
  round then takes 0.13–0.18 s instead of 0.35–0.43 s.

detekt's `ValidateClasspath` phase, which collects every file's
diagnostics and so waits for kc's analysis, takes 1.07–1.32 s on
kotlinpoet, against 1.38–1.59 s with the analysis in the background but
the PSI made first.

With all of it, `jvm/detekt/run.sh` with its defaults, 5 runs on a 4-core
machine after the run that writes the archive (`--analysis-mode full
--all-rules`; the findings were the same in every run, and the same as a
serial run's):

| Target | Stock, serial | Stock, `--parallel` | On kc-aa | Speedup |
|:---|---:|---:|---:|---:|
| kotlinpoet | 26.0 s | 18.5 s | 4.70–5.22 s, median 4.87 s | 5.3×, 3.8× |
| kotlinx-coroutines-core | 37.2 s | 29.5 s | 8.19–9.52 s, median 8.86 s | 4.2×, 3.3× |

That machine is slower than the one above: stock took 26.0 s on
kotlinpoet there, against 23.4 s.

## A second round, toward 5× stock `--parallel`

This round's goal was detekt on kc at least 5× faster than stock detekt
with `--parallel`. It got to 4.7× on kotlinpoet and 4.2× on
kotlinx-coroutines-core. The changes below were made on the branch
`work/detekt-5x`, one commit each. "Next" lists what is left.

| | kotlinpoet | kotlinx-coroutines-core |
|:---|---:|---:|
| Stock detekt, `--parallel` | 15.53 s | 24.23 s |
| detekt on kc, before this round | 3.81 s (4.1×) | 6.61 s (3.7×) |
| **detekt on kc, after it** | **3.28 s (4.7×)** | **5.71 s (4.2×)** |

- **How the rows were measured.** Stock against kc after the round is
  `bench/detekt-vs-stock.sh`: stock and kc round robin, the median of 5
  runs, with kc in 3.24–3.36 s and 5.56–5.86 s. Before against after is
  `bench/detekt-ab.sh`, with the build of the branch's base against its
  head, in one round robin: 3.81 → 3.38 s (−11%) and 6.61 → 5.79 s (−12%).
  The two rows come from round robins an hour apart, so their times differ
  slightly.
- **Findings.** kc's findings were the same in every run, serially and
  with `--parallel`. Change 3 removes 6 findings on coroutines that stock
  doesn't report. kc matches stock on 1581 of 1597 findings on
  kotlinpoet and on 2730 of 2778 on coroutines, as before.
- **The ratio depends on the machine's load.** The machine below, a cloud
  machine with 4 CPUs, ran much faster late in this round than early on.
  Stock took 29.3–33.5 s on kotlinpoet early on and 15.5 s late, 2.2×
  faster. The branch's base build of detekt on kc went from 6.19 s to
  3.81 s, only 1.6× faster. So speedups measured late in the round, as
  above, are lower than ones measured early on would be.
- **A measuring mistake.** For a while, the round robins ran under a
  watchdog that timed runs to the nearest 0.5 s. Those comparisons are
  left out. Every time here comes from exact runs.

### Setup

- A Linux x86_64 cloud machine with 4 CPUs and 15 GB, JDK 21.0.13, the
  JVM's default compilers.
- The targets, class paths and `-opt-in`s are those of the first round.
  The `-opt-in`s go after detekt's arguments, as `run.sh` takes them.
- The A/B runs are `bench/detekt-ab.sh`: variants round robin, 5 runs
  each, `--parallel`, each variant with its own class-data sharing
  archive. Only numbers from the same round robin are compared.

### Where the time went

At the base of the branch, one run on kotlinpoet with `--parallel`, under
JFR, with kc's timings (`KC_TIMING=1 -Dkc.trace=true`):

- **detekt waits for kc before any rule runs.** `ValidateClasspath`
  collects every file's diagnostics, so it waits for kc's analysis:
  1.5–1.7 s of a 6.8 s run on kotlinpoet, and 3.0–3.4 s on coroutines.
  kc's session has no diagnostics to add, as kc prints the errors it finds.
  Of that time:
  - kc's check took 1029 ms in the JVM, against 700 ms alone.
  - Waiting for the PSI after the check took 133 ms.
  - Building the round took 226 ms.
- **The JIT takes about two of the four cores.** C2's compilations took
  8.9 s of CPU in the 6 s run.
- **The rule threads block on the round's monitor.** About 1 s in total,
  in `KcRound.libraryClass`, `classSymbol` and `KcLint.modelSymbol`.
- **Each file was parsed twice, the first time serially.** One pool thread
  called `KtFile.isScript` on every file, which builds the file's stubs
  and so parses it. That was 49 of the pool's 98 samples, and it later
  made the rules switch each file's PSI from the stubs to the tree.
- **`--parallel` scales badly.** `Analyzer` goes from 6.1 s serially to
  3.4–4.0 s with `--parallel` (1.6×).

At the head of the branch, on coroutines:
- kc is 18% of the samples of detekt's rule threads.
- JIT compilations take 8.65 s of CPU. Of that, IntelliJ's parser is 3.7 s,
  detekt 1.5 s, kc 1.3 s, and the method handles of the `KaSession`
  proxy's reflective dispatch about 0.8 s.
- kc's check starts 0.59 s after the JVM, and takes 1.0–1.1 s in the JVM.
  Building the round then takes 0.27 s, and the first `analyze` call
  waits 0.95 s.

### 1. Read the round's symbols without a lock

**Evidence.** The ~1 s the rule threads spent blocked on the round's
monitor.

**Change.**
- `KcRound`'s maps are concurrent and read without a lock.
- Loading a library class or package, or building a member scope, still
  takes one lock, `loadLock`. A load registers its symbols in pending maps,
  which only the loading thread reads, and the outermost load publishes
  them when it ends. Until then, the symbols aren't complete.
- `KcLint.modelSymbol` and `KcAnalysis` no longer synchronize on the round.

**Result.** −0.20 s on kotlinpoet (6.05 → 5.85 s) and −0.52 s on
coroutines (9.73 → 9.21 s), early in the round.

**A deadlock.** Two runs out of about 500 hung. The rule threads were in a
lock-order cycle:
- one held a class symbol's lazy `superTypes`, whose class file
  `classFilePsi` looked up under the round's lock;
- another held that lock while building a member scope, which reads
  `superTypes`.

The cycle was there before, behind the round's monitor, but more paths
reached it concurrently. The first fix computed every lazy value under
`loadLock`, which made rule threads wait about 0.6 s for each other's
lazy values on coroutines.

The symbols' and types' lazy values are now
`LazyThreadSafetyMode.PUBLICATION`. No lock is held while one is computed,
so none can be part of a cycle, and every caller gets the value set first.
They make types and symbols of the model and register nothing.

### 2. Start kc's check first, and parse each file once

**Evidence.**
- `KcStandalone.Session` built IntelliJ's environment before it started
  kc's check, which needs only the sources' paths.
- The serial `isScript` calls above.
- At head, the check still started at the end of detekt's
  `CreateSettings`, 0.86 s after the JVM on coroutines.

**Change.**
- The session starts the check on a thread of its own before it builds
  its environment, and tells a script by its `.kts` extension.
- detekt patch `0005` starts the check in `Runner`, as soon as detekt-cli
  has parsed its arguments (`KcStandalone.prestart`). The session takes
  it over if it gets the same module: the same sources, JDK, class path,
  friend paths and opt-ins. Otherwise it checks again.
- `-Dkc.prestart=false` turns both off.

**Result.**
- Waiting for the PSI after the check went from 133 ms to 14 ms, and
  end to end −0.14 s on kotlinpoet and −0.86 s on coroutines, early in
  the round.
- Starting from `Runner` moved the check from 0.86 s to 0.59 s after the
  JVM, and `ValidateClasspath` on coroutines from 1.06 to 0.94 s. End to
  end that is within the noise: 5.60 → 5.58 s.

### 3. Don't look up class-file PSI to read supertypes

**Evidence.** A class symbol's `superTypes` asked for the class's PSI,
though it uses it only for a Java source's class. For a Java library
class, such as the JDK's, that is `JavaPsiFacade.findClass` plus building
the class file's stubs, under `loadLock`. At head on coroutines it was
20 of the 96 samples of the rule threads in kc, with 13 more in the class
files' stubs.

**Change.**
- `superTypes` asks only a Java source's class for its PSI.
- `KcInfo` keeps the PSI it found.
- `classFilePsi` reads its cache without a lock.

**Result.** Together with the lock-free lazies of change 1: −0.09 s on
kotlinpoet (3.27 → 3.18 s) and −0.21 s on coroutines (5.81 → 5.60 s).

### 4. Key library callees by their owner and flags

This fixes the findings rather than the time.

**Evidence.** With files analyzed in another order, coroutines lost 7
`UnnamedParameterUse` findings in every run. `KcLint` made one symbol for
each library callee, keyed by its name, parameter types and receiver.
The symbol answered from the call that made it, such as whether its
parameter names are stable, so whichever call a rule asked about first
decided it.

**Change.** The key includes the callee's owner and flags.

**Result.** The findings are the same in every order and serially.
Coroutines has 6 fewer findings, none of which stock reports.

### 5. Build the facts alongside the files' models

**Evidence.** After the check, `analysis/ka`'s `Update` built each file's
model and then the facts, one after the other on one goroutine:
133–185 ms and 120–265 ms cold.

**Change.** The facts are built on a goroutine of their own, while the
files' models are built on the session's jobs. `TestParallelModel` checks
that the encoded models and facts are the same byte for byte either way,
also on real sources. `KC_SERIAL_MODEL` builds them one after the other.

**Result.** The first, cold update (`TestLintStages`, median of 6) goes
from 744 to 614 ms on kotlinpoet and from 1128 to 951 ms on coroutines.
End to end it wasn't measured apart from the other changes.

### Off by default: overlapping the rules with kc's analysis

detekt patch `0004` lets the rules run while kc analyzes. With
`-Dkc.overlap=true`:
- `ValidateClasspath` is skipped, since kc prints its own errors.
- With `--parallel`, the rules that need no types run first, for every
  file (95 of about 300 rules implement `RequiresAnalysisApi`).
- Files go largest first, and a large file's rules are split over
  several tasks (`-Dkc.schedule=false` turns this off).

On 4 cores those rules and kc's check compete for the same CPUs, and it
doesn't pay:

| | kotlinpoet | coroutines |
|:---|---:|---:|
| At the start of the round | 5.22 → 5.21 s | 8.89 → 9.35 s |
| At the end of it | 3.26 → 3.23 s | 5.58 → 5.75 s |

### What isn't used

- **Compiling with C1 only** (`-XX:TieredStopAtLevel=1`). With
  `--parallel`, the rules, kc's check and the PSI parse need every core,
  and C2's compiler threads took about 9 s of CPU in a 6 s run. C1-only
  runs were clearly faster here, but that tunes the JVM rather than making
  detekt on kc faster, so `run.sh` doesn't use it. Its runs were among
  those timed to 0.5 s, so there is no exact figure.
- **A smaller pool for the PSI parse**, which competes with kc's check:
  5.63 s with 4 threads, against 5.52 s with 2, which is within the noise.

### Next

What is left, by the time it is expected to save on coroutines:

- **A persistent class-path cache for kc's Go check.** A cold check takes
  0.95–1.07 s and a warm one 0.66–0.75 s, and every detekt run is a cold
  process. −0.2 to −0.35 s.
- **kc's per-call costs in the session.** Callees' symbol keys are
  strings built on every call. Each fact is looked up through
  `containingFile` and two identity maps. −0.2 to −0.3 s.
- **Build the round's source symbols in parallel:** 0.27 s on one thread
  now. Class literals in annotations would have to be resolved lazily, so
  that a reference to another file's class doesn't depend on the order.
  −0.15 to −0.2 s.
- **A generated `KaSession` class instead of the `Proxy`.** This saves
  about 0.8 s of JIT CPU. −0.1 to −0.2 s.

### Reproducing

```
jvm/detekt/build.sh
ARGS="-i src/commonMain -i src/jvmMain --analysis-mode full --all-rules -cp ... --jdk-home $JAVA_HOME"
STOCK_JAR=detekt-cli-2.0.0-alpha.6-all.jar bench/detekt-vs-stock.sh /tmp/vs -- $ARGS
bench/detekt-ab.sh /tmp/ab "base|/tmp/base/libkc.so|-Dkc.overlap=false|/tmp/base/kc-aa.jar" \
	"head|jvm/kc-aa/build/libkc.so|" -- $ARGS --parallel
```

- A variant's fourth field names its `kc-aa.jar`, to compare builds of two
  commits.
- `KC_JAVA_OPTS="-XX:StartFlightRecording=filename=kc.jfr,settings=profile"`
  records a profile for `run.sh`. The JIT and monitor figures above come
  from `jdk.Compilation` and `jdk.JavaMonitorEnter` (1 ms threshold)
  events.

## Reproducing the first round

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
- On kc-aa, [`bench/detekt-ab.sh`](../bench/detekt-ab.sh) runs
  `jvm/detekt`'s build. A variant can name its own `kc-aa.jar`, as
  `"base|/tmp/base/libkc.so||/tmp/base/kc-aa.jar"`, to compare whole
  builds.
- `KC_LINT_SRC=src/commonMain:src/jvmMain KC_LINT_CP=... go test ./internal/kaops -run TestLintStages -v`
  times the Go stages of `OpKaUpdate` and `OpKaFacts`.
- `KC_JAVA_OPTS=-Dkc.trace=true jvm/detekt/run.sh ...` prints the
  repeat counts, and `KC_TIMING=1` kc's timings.
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
