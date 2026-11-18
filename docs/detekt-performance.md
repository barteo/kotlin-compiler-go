# Making detekt on kc faster

This page describes the five changes that most speed up
[detekt on kc](detekt.md), ranked by the time they save, with the
measurements behind each. All five are always on. They were measured one
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

- **C1 only** (`-XX:TieredStopAtLevel=1`) is often good for short JVM runs,
  but here it is slower: `Analyzer` takes 6.0–7.6 s with the archive,
  against 4.5–4.7 s. The rules run long enough for C2 to pay off.
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

## Toward 5× stock detekt with --parallel

The changes above took detekt on kc to 3.8× stock detekt with
`--parallel` on kotlinpoet and 3.3× on kotlinx-coroutines-core. The changes
in this section take it past 5× on both, with the same findings. The plan
was to make kc's session scale across the cores and to overlap kc's
analysis with the rules. The measurements showed that neither was the
bottleneck, so the plan changed.

### Setup

- The same targets, arguments and `-opt-in`s as [Setup](#setup), with
  `--parallel`, on a 4-core Linux x86_64 cloud machine, JDK 21.0.13. This
  machine is about twice as slow as the one above. Stock takes 35.8 s on
  kotlinpoet there (35.1–39.2 s, 4 runs), against 18.5 s, and 59.4 s on
  coroutines (56.6–59.7 s), against 29.5 s.
- 5× is therefore at most 7.16 s and 11.88 s.
- Each change was measured as an A/B against the previous one. Every
  variant had its own class-data sharing archive, and the runs alternated
  between variants: 5 or 7 runs of each, after a run that writes the
  archive. Every run's findings (file, line, column and rule) were checked
  against the first variant's.
- Runs on this machine vary by ±1 s, so a median moves by up to half a
  second between batches.

### Where the time goes now

The run is CPU-bound. JFR's CPU load shows the machine 97–99% busy from
the first second to the last, the time detekt waits for kc's analysis
included. Each thread's CPU time, read from `/proc` during one run on
coroutines, before these changes and after them:

| Threads, coroutines | Before | After |
|:---|---:|---:|
| C2 and C1, the JIT compilers | 24.6 s (46%) | 17.1 s (39%) |
| detekt's rules (`pool-1-thread-*`) | 17.8 s (34%) | 17.9 s (41%) |
| `kc-pool`: the PSI, the facts' decoding | 3.9 s (7%) | 2.4 s (5%) |
| `kc-analysis` and the Go threads it starts | 3.8 s (7%) | 4.0 s (9%) |
| The main thread, GC and the rest | 2.9 s (5%) | 2.3 s (5%) |
| **Total** | **53.1 s** | **43.7 s** |

- Of the rules' samples in JFR (2 ms period, full stacks), 51% were the
  walk of the tree itself: every rule walks every file, as a
  `KtTreeVisitorVoid`. 18% were in kc's session, spread thin over many
  questions. No rule took more than 4%.
- 381 of `kc-pool`'s 511 samples were in one task: `KtFile.isScript`.
- The run's time is its CPU time divided by four. Overlapping work and
  removing locks can't save time unless they also save CPU.

### The changes, ranked by the time they save on coroutines

| # | Change | coroutines | kotlinpoet |
|:---|:---|---:|---:|
| 1 | Don't build every file's stub tree | 14.56 → 12.77 s | |
| 2 | The rules' walks step over the leaves (detekt patch 0005) | 13.65 → 12.21 s | 7.79 → 6.86 s |
| 3 | The parallel collector (`run.sh`) | 11.73 → 10.87 s | 6.82 → 6.52 s |
| 4 | Skip ValidateClasspath and run the PSI-only rules first (patch 0004) | 13.66 → 13.65 s | |
| 5 | Read the round without its lock | 14.56 → 14.36 s | 8.26 → 8.20 s |

**1. Don't build every file's stub tree.** `KcStandalone` filtered the
files with `KtFile.isScript`, which builds the file's stub tree: it parses
the file and builds its stubs. That happened for every file, one after
another, in one task on the pool, before the round could be built. With
the stubs built, every stub-based element also reached its AST node through
the file's spine (`SpineRef`), on each `getFirstChild` and `getNextSibling`
of every rule's walk. A script is a `.kts` file, so the extension says it.
kc's `psi=` time after the update went from 1188 ms to 4 ms on coroutines,
and `ValidateClasspath` from 3.87 s to 2.74 s.

**2. Step over the leaves.** Most of a tree's elements are leaves: tokens,
white space and comments. A leaf is visited only by `visitElement`,
`visitWhiteSpace`, `visitComment` and `visitPlainText`. A visitor that
overrides none of them sees nothing of the leaves. `DetektVisitor`'s walk
now steps over them, by the AST. Of the rules built here, only `NoTabs` and
`ForbiddenComment` visit leaves, and the lines-of-code processor does too.
They still walk everything, as does any visitor with
`-Ddetekt.visitLeaves=true`. This change helps stock detekt too.

**3. The parallel collector.** In a CPU-bound run, G1's write barriers and
concurrent threads cost the rules' threads more than the parallel
collector's pauses do. `run.sh` passes `-XX:+UseParallelGC` unless
`KC_JAVA_OPTS` names a collector.

**4. Skip ValidateClasspath and run the PSI-only rules first.** On kc,
`collectDiagnostics` has nothing to collect: kc reports the errors it finds
itself. So ValidateClasspath only waited for kc's analysis, and it is left
out. The rules then run in two waves: the rules that read only the PSI,
while kc analyzes, and then those that read the Analysis API. On 4 cores
this saves nothing, because the cores were busy during the wait too. It is
kept because the phase did no work. With more cores than work, the PSI-only
rules hide kc's analysis.

**5. Read the round without its lock.** Every symbol lookup took
`KcRound`'s monitor, and library loads made JNI calls under it. In JFR the
rules' threads waited 0.5 s in all on it on kotlinpoet. Removing it saves
nothing measurable. It also fixes real races: the round's `HashMap`s were
read without the lock while a load wrote them. Concurrent maps are now read
without the lock, and a library class, package, typealias or member scope
is taken by another thread only once its load is done.

### Also on the branch

- **Building the files' models on all cores, and the facts alongside**
  (`ka`, from `work/detekt-5x`). `TestParallelModel` checks, under `-race`
  and on both targets, that the models and facts are the same byte for
  byte. On 4 cores it is within the noise: kc's analysis isn't what the run
  waits for.
- **Library callees keyed by their owner and flags** (also from
  `work/detekt-5x`). Analyzing the largest files first changed the
  findings: callees with the same name and parameter types shared the
  symbol of whichever call a rule asked about first. With the owner and
  flags in the key, coroutines has 2757 findings serially, with
  `--parallel` and without the waves. The 6 that went away are ones stock
  doesn't report. 2743 match stock's, before and after.
- **Path filters made once** (detekt patch 0006): about 100,000 glob
  compilations, within the noise.

### What didn't help

- **Analyzing the largest files first.** On kotlinpoet the rules' threads
  were parked 1.5 s in all at the end of the run, waiting for the last
  files. Submitting the largest first saved nothing measurable, and it
  exposed the callee-key bug above. It isn't on the branch.
- **The JIT compilers** use 39–45% of the CPU. Changing them (C1 only, or
  fewer compiler threads) was left out of this work.

### Result

`jvm/detekt/run.sh` with its defaults, 5 runs after the run that writes
the archive, against stock with `--parallel` on the same machine:

| Target | Stock, `--parallel` | On kc, before | On kc, after | Speedup |
|:---|---:|---:|---:|---:|
| kotlinpoet | 36.7 s (35.1–40.8, 7 runs) | 8.24 s | 6.48 s (5.85–7.25, 7 runs) | 4.5× → **5.7×** |
| kotlinx-coroutines-core | 59.0 s (53.8–60.5, 7 runs) | 14.28 s | 10.94 s (10.08–11.70, 7 runs) | 4.1× → **5.4×** |

- "Before" is `bf651a6` on the same machine, with a valid archive.
- The findings were the same in every run: 1601 on kotlinpoet, and 2757
  on coroutines, which is 6 fewer than before because of the callee keys.
- Against only the 3 stock runs that were interleaved with these runs, the
  speedups are 5.7× and 5.2×.
- The kc-aa KSP suites (`jvm/kc-aa/build.sh test`) and the fixture
  (`jvm/detekt/build.sh test`) pass.

To compare builds, keep each one's `libkc.so`, `kc-aa.jar` and `detekt.jar`,
and give [`bench/detekt-ab.sh`](../bench/detekt-ab.sh) one variant per
build, with `--parallel` among detekt's arguments:

```
RUNS=7 bench/detekt-ab.sh /tmp/ab \
	"before|/tmp/before/libkc.so|-XX:SharedArchiveFile=/tmp/b.jsa -XX:+AutoCreateSharedArchive|/tmp/before/kc-aa.jar|/tmp/before/detekt.jar" \
	"after|jvm/kc-aa/build/libkc.so|-XX:+UseParallelGC -XX:SharedArchiveFile=/tmp/a.jsa -XX:+AutoCreateSharedArchive" \
	-- $ARGS --parallel
```

`-Dkc.waves=false`, `-Dkc.validateClasspath=true` and
`-Ddetekt.visitLeaves=true` turn off patches 0004 and 0005. Each thread's
CPU time comes from `/proc/<pid>/task/*/stat` during the run, and the
profiles from JFR with `-XX:FlightRecorderOptions:stackdepth=1024`. Rules'
walks are deeper than JFR's default of 64 frames.

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
