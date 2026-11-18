# Making detekt on kc faster

This page describes the changes that speed up [detekt on kc](detekt.md),
with the measurements behind each, in two rounds. The first five took
detekt on kc to 3.8× and 3.3× stock detekt with `--parallel`.
[Five more](#five-more-5-stock---parallel) took it past 5×: 7.1× on
kotlinpoet and 7.0× on kotlinx-coroutines-core.

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
  takes 6.0–7.6 s with the archive, against 4.5–4.7 s, as C2's compiler
  threads have idle cores to use. With `--parallel` it is the other way
  round, and `run.sh` now uses it ([C1 only](#2-compile-with-c1-only)).
- **Replacing the `KaSession` proxy** with generated code isn't worth it.
  Its reflective dispatch is 0.03 s of the run, for about 300,000 calls.

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

## Five more: 5× stock `--parallel`

The goal of this round was detekt on kc at least 5× faster than stock
detekt with `--parallel`. These five changes, each with evidence from
profiles of detekt on kc-aa, got it there. They were made on the branch
`work/detekt-5x`, one commit each.

| | kotlinpoet | kotlinx-coroutines-core |
|:---|---:|---:|
| Stock detekt, `--parallel` | 33.46 s | 52.21 s |
| detekt on kc before this round | 5.56 s (6.0×) | 11.14 s (4.7×) |
| **detekt on kc after it** | **4.72 s (7.1×)** | **7.47 s (7.0×)** |

- The first two rows come from separate round robins on the same machine.
  Stock vs kc is `bench/detekt-vs-stock.sh`. Before vs after is
  `bench/detekt-ab.sh`, with the build of the branch's base against its
  head, which gives 5.56 → 4.04 s and 11.14 → 7.59 s.
- Each time is the median of 5 runs. Even kc's slowest run against
  stock's fastest is 6.5× and 5.8×.
- kc's findings were the same in every run, serially and with
  `--parallel`. On coroutines, change 5 below removes 6 findings that
  stock doesn't report. kc matches stock on 1581 of 1597 findings on
  kotlinpoet and 2730 of 2778 on coroutines, as before.

### Setup

- A Linux x86_64 cloud machine with 4 CPUs and 15 GB, JDK 21.0.13. It is
  slower than the machine of the first round: stock with `--parallel`
  took 29.3–33.5 s on kotlinpoet here, against 18.5 s there.
- The targets, class paths and `-opt-in`s are those of the first round.
  The `-opt-in`s go after detekt's arguments, as `run.sh` takes them.
- Every A/B below is `bench/detekt-ab.sh`: variants round robin, 5 runs
  each, `--parallel`, each with its own class-data sharing archive. The
  machine drifted by up to 20% over the hours of these runs, so only
  numbers from the same round robin are compared.

### Where the time went

One run on kotlinpoet with `--parallel`, at the base of the branch, under
JFR, with kc's timings (`KC_TIMING=1 -Dkc.trace=true`):

- **detekt waits for kc before any rule runs.** `ValidateClasspath`
  collects every file's diagnostics, so it waits for kc's analysis:
  1.5–1.7 s of a 6.8 s run on kotlinpoet, 3.0–3.4 s on coroutines. kc's
  session has no diagnostics to add, since kc prints the errors it finds.
  Of that time:
  - kc's check took 1029 ms in the JVM, against 700 ms alone, because the
    JVM's threads compete with it for the cores.
  - Waiting for the PSI after the check took 133 ms.
  - Building the round took 226 ms.
- **The JIT takes two of the four cores.** C2's compilations took 8.9 s of
  CPU in the 6 s run, on two compiler threads. The largest compiled methods
  were kc's own: `KcLintAnalysis.referenced`, `KcAnalysis.resolveToSymbol`
  and `lintLibraryCallableId`, each 40–73 KB of code. So were the method
  handles of the `KaSession` proxy.
- **The rule threads block on the round's monitor.** They waited about 1 s
  in total, in `KcRound.libraryClass`, `classSymbol` and
  `KcLint.modelSymbol`. Every lookup took the monitor, also for what was
  already loaded.
- **Each file was parsed twice, the first time serially.** One pool thread
  called `KtFile.isScript` on every file before the pool parsed them.
  That builds the file's stubs, which parses it. It was 49 of the pool's
  98 samples, and it later made the rules switch each file's PSI from the
  stubs to the tree.
- **`--parallel` scales badly.** `Analyzer` goes from 6.1 s serially to
  3.4–4.0 s with `--parallel` (1.6×). In light mode, which needs no
  types, it goes from 4.2 s to 3.5 s (1.2×), because kc's analysis runs
  meanwhile and takes the cores.

### 1. Start kc's check first, and parse each file once

**Evidence.**
- `KcStandalone.Session` built IntelliJ's environment before it started
  kc's check. That environment is `KotlinCoreEnvironment` and the class
  path's index, and the check needs neither: it reads the sources from
  their paths.
- The serial `isScript` calls above.

**Change.**
- The session starts the check on a thread of its own before it builds
  its environment. `-Dkc.prestart=false` starts it after, as before.
- A script is told by its extension, `.kts`.
- `-Dkc.trace` prints when the check started and how long the first
  `analyze` call waited for the analysis.

**Result.** Waiting for the PSI after the check went from 133 ms to 14 ms.
End to end, −0.14 s on kotlinpoet (6.19 → 6.05 s) and −0.86 s on
coroutines (10.59 → 9.73 s).

### 2. Compile with C1 only

**Evidence.** The JIT's 8.9 s of CPU above. The rules, kc's check and the
PSI pool need all four cores, so C2's compiler threads slow them down
more than C2's code speeds them up.

**Change.** [`run.sh`](../jvm/detekt/run.sh) passes
`-XX:TieredStopAtLevel=1`. `KC_JAVA_OPTS=-XX:TieredStopAtLevel=4` turns C2
back on.

**Result.** −1.54 s on kotlinpoet (6.59 → 5.05 s, −23%) and −3.05 s on
coroutines (11.64 → 8.59 s, −26%). This is the largest of the five. It
also shortens `ValidateClasspath`, from 2.22 s to 1.55 s on coroutines,
because kc's check gets more of the CPU. The first round measured C1 as
slower, but serially.

### 3. Run the rules that need no types while kc analyzes

**Evidence.** The waits in `ValidateClasspath` above. Of detekt's roughly
300 rules, 95 implement `RequiresAnalysisApi`. The rest need no types and
can run before kc is done.

**Change.** detekt patch
[`0004`](../jvm/detekt/patches/0004-Run-the-rules-that-need-no-types-while-kc-analyzes.patch):
- `ValidateClasspath` is skipped when the session reports its own errors
  (`EnvironmentAware.reportsOwnErrors`).
- With `--parallel`, the `Analyzer` first runs the rules that need no
  types, for every file, and then the ones that need types.
- A file's issues keep a serial run's order, and its processors see the
  file once, with all of its issues.
- With `--auto-correct`, which changes the PSI, each file's rules still
  run in one task.
- `-Dkc.overlap=false` turns it off.

**Result.** It depends on the JIT, because the overlap only helps when
there are cores to spare:

| | kotlinpoet | kotlinx-coroutines-core |
|:---|---:|---:|
| C2 | 6.59 → 6.61 s | 11.64 → 12.20 s |
| C1 only (change 2) | 5.05 → 4.56 s (−10%) | 8.59 → 7.60 s (−12%) |

### 4. Read the round's symbols without its lock

**Evidence.** The ~1 s the rule threads spent blocked on the round's
monitor.

**Change.**
- `KcRound`'s maps are concurrent and read without a lock.
- Loading a library class or package, or building a member scope, still
  takes one lock, `loadLock`. A load registers its symbols in pending maps,
  which only the loading thread reads, and the outermost load publishes
  them when it ends. Until then, the symbols aren't complete.
- `KcLint.modelSymbol` and `KcAnalysis` no longer synchronize on the round.

This also ends some unsynchronized reads of `byPsi`, and of the maps that
library loads write, which other threads made while one thread loaded.

**Result.** −0.20 s on kotlinpoet (6.05 → 5.85 s) and −0.52 s on
coroutines (9.73 → 9.21 s), with C2.

**A deadlock.** Two runs out of about 500 hung during these measurements.
The rule threads were stuck in a lock-order cycle:
- one held a class symbol's lazy `superTypes`, whose class file
  `classFilePsi` looks up under the round's lock;
- another held that lock while building a member scope, which reads
  `superTypes`.

The cycle was there before, behind the round's monitor, but more paths
now reach it concurrently. Every lazy value of the round's symbols and
types is now computed under the round's `loadLock`, so one lock orders
them. After that, 115 runs under a watchdog didn't hang.

### 5. Key library callees by their owner and flags

This one fixes the findings rather than the time. The scheduling it
was found with saves nothing measurable (see below).

**Evidence.** With files analyzed largest first instead of in path order,
coroutines lost 7 `UnnamedParameterUse` findings, in every run. `KcLint`
made one symbol for each library callee, keyed by its name, parameter
types and receiver, and the symbol answered from the call that made it,
for example whether its parameter names are stable. An invoke of a
function-typed value and another call with the same key shared the
symbol of whichever call a rule asked about first.

**Change.** The key includes the callee's owner and flags.

**Result.** The findings are the same in path order, largest first and
serially. Coroutines has 6 fewer findings, none of which stock reports.

### What didn't pay off end to end

- **Largest files first, and splitting large files' rules over several
  tasks** (in patch 0004, `-Dkc.schedule=false` to turn it off): 4.55 vs
  4.56 s and 7.60 vs 7.60 s. No file is large enough to leave a thread
  alone at the end on these targets.
- **Building the facts alongside the files' models, and the models on all
  cores** (`analysis/ka`, `KC_SERIAL_MODEL` to turn it off). The first,
  cold update gets faster in `TestLintStages`: 744 → 614 ms on kotlinpoet
  and 1128 → 951 ms on coroutines. End to end it is 4.55 vs 4.55 s and
  7.61 vs 7.60 s, because the rules that need no types now run
  meanwhile. `TestParallelModel` checks that the encoded models and facts
  are the same byte for byte either way.

Both stay in. They don't change the findings, and they would matter on a
module with one very large file, or with fewer rules to overlap with.

### Reproducing

```
jvm/detekt/build.sh
ARGS="-i src/commonMain -i src/jvmMain --analysis-mode full --all-rules -cp ... --jdk-home $JAVA_HOME"
STOCK_JAR=detekt-cli-2.0.0-alpha.6-all.jar bench/detekt-vs-stock.sh /tmp/vs -- $ARGS
bench/detekt-ab.sh /tmp/ab "c2|jvm/kc-aa/build/libkc.so|-Dkc.overlap=false" \
	"c1|jvm/kc-aa/build/libkc.so|-XX:TieredStopAtLevel=1" -- $ARGS --parallel
```

- A variant's fourth field names its `kc-aa.jar`, to compare builds of two
  commits.
- `KC_JAVA_OPTS="-XX:StartFlightRecording=filename=kc.jfr,settings=profile"`
  records a profile for `run.sh`. The JIT and monitor figures above come from
  `jdk.Compilation` and `jdk.JavaMonitorEnter` (1 ms threshold) events.

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
