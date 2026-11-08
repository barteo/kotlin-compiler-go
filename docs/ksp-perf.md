# KSP2 on kc: the plan to 10x

The goal is KSP2 on the `kc` backend running at least 10x faster than stock
KSP2, whose Analysis API runs on kotlinc's FIR in standalone mode. Both run
the same processors, unchanged, on the same JVM. This plan covers KSP2
only; KSP1 is out of scope. Background and history are in
[analysis-api.md](analysis-api.md) (steps 11, 17, 24, 25).

## Where we are

octi-server (Dagger, 60 Kotlin files, 31 generated Java files), 4 CPUs,
JDK 21, `bench/ksp/run.sh` (steps 11 and 17):

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| warm run, nothing changed | 773 | 176 (setup 19, process 130, between rounds 17–28) | 4.4x |
| cold JVM | 6190–6440 | 3470–3610 | 1.8x |
| cold JVM, CDS archive (step 11) | 5810–6130 | 1870–2110 | 3x |
| one-file body edit, Go update only | – | 25 (45 if a declaration changes) | – |

These numbers predate steps 18–25.

TachiyomiExhaustive, where kc's dump of what the processor sees equals
stock's except for the differences step 26 lists. Cold is from step 26,
warm from step 27, and edits from step 28:

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| cold JVM, first run | 23184 | 5882 | 3.9x |
| Hot (fastest of runs 4–6) | 6570 | 308 | 21x |
| warm (median) | 6764 | 308 | 22x |
| one-file body edit (median) | 871 | 204 | 4.3x |
| one-file signature edit (median) | 881 | 296 | 3.0x |

W is past 10x. E is at 3–4x: an edit still lowers all 559 files and
resolves their signatures in Go (100–210 ms), and kc-aa builds a new
round for every file (30–60 ms). Step 28 lists what is left.

Room (step 29), on a generated module of 100 files with 200 entities and
100 DAOs: warm runs take 8104 ms on stock and 3716 ms on kc (2.2x), and
edits 9655 and 4877 ms (2.0x). Room's own processing takes about 2.7 s
a run on both backends, so even a backend that took no time couldn't
get better than about 2.9x. On kc, the backend's share is about 1.3 s,
against 6.1 s on stock.

Hilt (step 30), on a generated module of 100 files (factories,
multibindings, entry points, assisted injection): warm runs take 4415 ms
on stock and 1486 ms on kc (3.0x), and edits 5294 and 2396 ms (2.2x). On
kc, Hilt itself and writing its 1260 files take about 77% of a run, and
the backend about 8%.

## What "10x" has to mean

**The processor floor.** In step 11's warm JFR, Dagger, XProcessing and
JavaPoet/google-java-format took about 62% of kc's `process` (about 80 ms).
That code runs the same on both backends. Stock's warm octi run is 773 ms,
so 10x is 77 ms, below what Dagger alone costs. On a processor-heavy
workload, total wall time can't reach 10x however fast kc is. So the
targets are set per scenario and per workload, and the processors' own time
is always reported beside the totals.

**Scenarios.** In Gradle, a `kspKotlin` with no inputs changed is
UP-TO-DATE and doesn't run, so a warm run with nothing changed isn't what
users wait for. The scenarios that matter:

| | scenario | what it is in Gradle | 10x target |
|---|---|---|---|
| **E** | edit | a warm daemon after a one-file edit, incremental KSP on (body edit, and an edit that changes a declaration) | yes, the headline |
| **W** | warm full | a warm daemon, every file processed (`--rerun-tasks`, a changed class path, a non-incremental processor) | yes, on KSP-bound workloads |
| **C** | cold | the first `kspKotlin` of a new daemon, and CI | 10x against stock with default JVM flags; also reported against stock with the same AOT/CDS flags, where the processors' warm-up is a shared floor |

Stock KSP2 makes a new standalone AA session every run and has no
module-level reuse between runs, so its edit run costs about as much as its
warm full run. That makes E the scenario where kc's kept state wins most.

**Workloads.**
- **Exhaustive** (KSP's own `TachiyomiExhaustive` benchmark: 559 files, 136
  jars, a processor that visits every node through the KSP API). It
  measures KSP plus the backend with almost no processor floor. This is
  the primary workload. Inputs are fetched as upstream's CI does
  (`bench/ksp/exhaustive/fetch.sh`, in progress in the working tree).
- **Moshi** and **Room** fixtures (`jvm/kc-aa/test`), scaled up, with light
  processors.
- **octi-server** (Dagger): heavy processors. This one shows the floor.

## Phase 0: measure (do first)

1. **Exhaustive in `run.sh`**, on both backends, compiled against the KSP API.
2. **Edit mode in `KspBench`**: a run is preceded by a one-file mutation
   (a body statement, then a signature change, alternating), incremental
   KSP with its caches directory kept between runs, generated output
   written in place, as Gradle does. Report the dirty-file count KSP
   computes on each backend; it must match.
3. **Split every number three ways**: processor code, KSP's symbol layer, and
   kc (the kc-aa JVM layer, Go, IntelliJ PSI). `Classify.java` already
   classifies by leaf frame. Add an inclusive view (time under a KSP/kc
   frame called from a processor frame counts as platform, not processor).
4. **Re-baseline** W, E and C on all workloads at this branch, with and
   without CDS. Record the per-workload ceiling: stock total divided by
   kc's processor floor.
5. **Spot-check against real Gradle**: one `kspKotlin` on octi with the
   fork, to confirm the harness matches the task within about 10%.

Exit: a table of W, E and C per workload with the three-way split, and a
ceiling for each. That table decides where Phases 1–3 aim first.

## Phase 1: warm full (W)

Target: kc's platform time (KSP layer plus kc) at most a tenth of stock's
total, minus the floor. For Exhaustive, that is the whole 10x.

- **`getSymbolsWithAnnotation` and `getAllFiles` from the model.** KSP walks
  every file's symbols to find annotated ones. Answer the walk from an
  annotation index built in Go (the model already carries annotations), and
  materialize only the matching symbols.
- **Lazy `KcRound`.** It maps every file's kc nodes to PSI up front
  (`byKey`, `byPsi`). Build a file's tables on first touch instead.
- **Keep the decoded model across runs.** When Go returns a kept check
  (`reused`), keep `KcModel` and the round's symbols on the JVM instead of
  encoding and decoding again (4 ms per round today). Also keep decoded
  library classes per library key, so a warm run makes no `KcOps.cls`
  calls for classes it has seen.
- **KSP hot paths the processors call:** `asMemberOf`,
  `isAssignableFrom`, `KSType` equality and hashing (`KcSubstitution`,
  `KcTypes`), `getJvmName`/`mapToJvmSignature`, and repeated
  `getClassDeclarationByName`. Cache per round and batch JNI lookups. Use
  Phase 0's inclusive profile to pick them.
- **Locations without PSI documents.** Exhaustive reads `location` for every
  node. Give line numbers from kc's line tables.
- **Between rounds (17–28 ms):** find what it is and remove it.

## Phase 2: edits (E)

Target: kc's edit run at most a tenth of stock's edit run on every
workload, octi included. The dirty set is small, so the floor shrinks.

- **The dirty set.** Check that KSP's IC on kc (lookups replayed, step 16)
  reprocesses the same files as stock and no more. Over-dirtying wastes
  processor time we can't speed up.
- **Go: the per-module rest.** Lowering and written-signature resolution
  still run for every file (about 12 ms of the 25 on octi, step 25). Make
  written types resolve lazily, as omitted ones already do.
- **Go: declaration edits without a whole check.** Use the lookup graph
  kc already records (step 16): check the edited file plus the files whose
  recorded lookups name a declaration whose signature changed. Verify each
  case against a whole check, as `TestIncremental` does.
- **A model delta over JNI.** Encode and decode only the changed files;
  keep the others' decoded `KcFile`s.
- **JVM: incremental rounds.** Re-map only the changed file's PSI, and keep
  the unchanged files' symbol tables under the new round's lifetime token.
  Check that KSP's symbol identity rules across runs allow it.

## Phase 3: cold (C)

Step 11's split of kc's cold 3.5 s: environment 500 ms, first Go update
280–310 ms, first `KcRound` 500–660 ms (IntelliJ parsing PSI with code the
JIT hasn't compiled yet), processing 1.3 s (64% the processors' own class
loading and warm-up).

- **A slim environment.** Replace `KotlinCoreEnvironment` with the minimum
  KSP needs: the Kotlin PSI parser, plus Java PSI and the file manager only
  when the module has Java sources. Target: under 100 ms.
- **Lazy PSI.** Parse a file's `KtFile` only when KSP asks for its PSI
  (`psi`, origin checks, PSI-based locations), not for every file at round
  start. List what in the fork's KSP layer forces PSI, and serve the rest
  from the model. Target: first round under 100 ms.
- **A persistent class-path index.** Write the shared library (class
  headers, decoded Kotlin metadata) to disk, keyed by jar stamps as
  `libcache.go` keys it, and map it in on start. Target: first update
  under 60 ms.
- **JVM AOT.** Use JDK 25's AOT cache with profiles (JEP 483/514/515) as the
  successor to step 11's CDS archive: it also cuts the processors' warm-up.
  Document or automate it for the Gradle daemon (`org.gradle.jvmargs`).
  Measure stock with the same flags, and report both ratios.
- Expected: 3.5 s down to about 0.6–0.9 s with AOT, about 10x stock with
  default flags. Against stock with the same AOT flags, the shared
  processor warm-up probably caps the ratio near 5–6x. Phase 0's numbers
  will firm this up.

## Phase 4: keep it

- Run Exhaustive in CI on every PR (inputs cached), reporting W and E for
  both backends. Not gating, because shared runners are noisy, but tracked.
- Conformance stays green after every step: `build.sh stock` (byte-identical
  output for Moshi, Dagger, Room, Hilt), `build.sh ksp` (134 pass),
  kotlinpoet (18 of 19), `TestIncremental*`.
- Write each step up in `analysis-api.md`, as before.

## Order and expected effect

| order | work | scenario | expected |
|---|---|---|---|
| 1 | Phase 0 | all | the real ceilings; may reorder the rest |
| 2 | model-backed annotation walk, lazy `KcRound`, kept decoded model | W, E | most of kc's JVM-side warm cost |
| 3 | dirty-set parity, model delta, incremental rounds | E | edit run dominated by the processors' work on the dirty files |
| 4 | lazy PSI, slim environment | C (and W) | about 1 s of cold setup |
| 5 | persistent class-path index, AOT cache | C | about 0.3 s plus the processors' warm-up |
| 6 | Go: lazy written types, lookup-driven declaration edits | E | 25 to about 10 ms on body edits; 45 to about 15 ms on declaration edits |
| 7 | KSP hot paths from profiles | W | depends on the processor |

## Risks

- **The processor floor.** On Dagger-like workloads, W can't reach 10x on
  total time. E and C can, if the dirty set stays small and the processors'
  warm-up is shared through AOT.
- **The harness versus Gradle.** `KspBench` drives `KotlinSymbolProcessing`
  directly. Gradle adds task and worker overhead that is the same on both
  backends, which lowers the ratio. Phase 0.5 measures it.
- **Laziness and conformance.** Lazy PSI and incremental rounds change when
  and how symbols are made. KSP's tests and the byte-identical processor
  outputs must catch any drift.
- **One libkc per daemon** (step 21) is unchanged by this plan.
