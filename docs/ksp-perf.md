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

Room, Hilt and Moshi (steps 29–42), on generated modules of 100 files,
after step 36, with step 39's leaf edits. Ceiling means the ratio if the
backend took no time: stock's run divided by the processors' own time,
which is the same on both backends. Rows are medians of 10 runs, but for
the leaf edits, which are the medians of runs 51–100 of 100 on each
backend: both JITs keep warming up well past 30 runs (step 38). Stock
ran Moshi's leaf edits in about 200 ms in step 39's session, against 254
in step 38's, and in 238 and 188 ms in two batches one after the other in
step 42's: Moshi's step 42 rows pool both batches.

| ms | stock KSP2 | kc | ratio | ceiling |
|---|---|---|---|---|
| Room, warm | 7067 | 2514 | 2.8x | about 3.5–4x (Room, XProcessing and KotlinPoet are about 90% of kc's run) |
| Room, leaf edit | 8331 | 2998 | 2.8x | about 3.5x |
| Room, leaf body edit (step 58) | 9054 | 3076 | 2.9x | |
| Room, leaf signature edit (step 58) | 8967 | 2870 | 3.1x | |
| Hilt, warm | 3916 | 692 | 5.7x | about 10x |
| Hilt, edit Feature000 (100 files dirty) | 4113 | 1161 | 3.5x | |
| Hilt, leaf body edit (step 42) | 476 | 38 | 12.5x | about 20x (processing takes about 20 ms) |
| Hilt, leaf signature edit (step 42) | 487 | 66 | 7.4x | |
| Hilt, leaf signature edit (step 43, slower machine) | 572 | 86 | 6.7x | step 43 cut 9 ms (A/B on one machine) |
| Hilt, leaf body edit (step 44, slower machine) | 505 | 43 | 11.7x | |
| Hilt, leaf signature edit (step 44, slower machine) | 495 | 54 | 9.2x | steps 43–44 cut 20 ms (A/B on one machine) |
| Hilt, leaf body edit (step 45, slower machine) | 504 | 41 | 12.3x | |
| Hilt, leaf signature edit (step 45, slower machine) | 481 | 48 | 10.0x | step 45 cut 5–6 ms (A/B against step 44's libkc) |
| Hilt, leaf body edit (step 46, Gradle's layout) | 484 | 36 | 13.4x | |
| Hilt, leaf signature edit (step 46, Gradle's layout) | 476 | 43 | 11.1x | |
| Hilt, leaf body edit (step 47) | 461 | 37 | 12.5x | step 47 cut 3.5 ms (A/B, 34.5 against 38) |
| Hilt, leaf signature edit (step 47) | 460 | 44 | 10.5x | step 47 cut 3 ms (A/B, 41 against 44) |
| Hilt, leaf body edit (step 48) | 366 | 34 | 10.8x | stock's teardown was 120 ms here, 185 in step 47; step 48 cut 1–1.5 ms (A/B) |
| Hilt, leaf signature edit (step 48) | 376 | 40 | 9.4x | the skip doesn't fire on Hilt's signature edits |
| Hilt, leaf body edit (step 49) | 469 | 40 | 11.7x | this session's machine was slower: kc's Moshi body run took 21 ms against step 48's 18 |
| Hilt, leaf signature edit (step 49) | 477 | 47 | 10.1x | step 49 cut 0.4 ms off the two updates (A/B), inside the totals' spread |
| Hilt, leaf body edit (step 50) | 325 | 34 | 9.6x (13.8x against step 49's stock) | stock's teardown was 78 ms here, 190 in step 49; this batch ran about 15% faster on both backends |
| Hilt, leaf signature edit (step 50) | 334 | 39 | 8.6x (12.2x against step 49's stock) | step 50 cut 0.4 ms off the two updates (A/B), inside the totals' spread |
| Hilt, leaf body edit (step 51) | 335 | 35 | 9.6x | stock's teardown 93 ms; step 51 took nothing off Hilt (A/B) |
| Hilt, leaf signature edit (step 51) | 335 | 40 | 8.4x | |
| Hilt, leaf body edit (step 52) | 368 | 35 | 10.5x | stock's teardown 112–115 ms |
| Hilt, leaf signature edit (step 52) | 378 | 39 | 9.7x | step 52 cut 2 ms (A/B, 40 against 42; round 2's update 1.8 against 3.2 ms) |
| Hilt, leaf body edit (step 54) | 337 | 36 | 9.4x (10.2x against step 52's stock) | stock's teardown 65 ms, 112–115 in step 52 |
| Hilt, leaf signature edit (step 54) | 334 | 39 | 8.6x (9.7x against step 52's stock) | step 54 cut the teardown 0.6–0.9 ms (A/B, equal environments) |
| Hilt, leaf body edit (step 55, two batches) | 355.5 | 37 | 9.6x | stock's teardown 66–166 ms across this step's four batches: 8.8x to 11.8x |
| Hilt, leaf signature edit (step 55, two batches) | 359 | 39.5 | 9.1x | step 55 cut `between` 1–2 ms (A/B, equal environments) |
| Hilt, leaf body edit (step 57) | 401 | 35 | 11.5x (10.2x against step 55's stock) | stock's teardown 120 ms, setup 63 ms |
| Hilt, leaf signature edit (step 57) | 401 | 37 | 10.8x (9.7x against step 55's stock) | step 57 cut round 1's read 0.6 ms (A/B, equal environments; totals within the spread) |
| Hilt, leaf body edit (step 58) | 436 | 31 | 14.1x (11.5x against step 55's stock) | stock's teardown 182 ms |
| Hilt, leaf signature edit (step 58) | 418 | 33 | 12.7x (10.9x against step 55's stock) | step 58 cut 24 `KcOps.cls` calls, 0.5 ms (A/B, equal environments) |
| Hilt, one-file test, warm | 403 | 59 | 6.8x | |
| Moshi, warm | 2270 | 458 | 5.0x | about 7.5x |
| Moshi, edit Model000 (10 files dirty) | 610 | 167 | 3.7x | |
| Moshi, leaf body edit (step 42, two batches) | 213 | 20.5 | 10.4x (9.4–11.3x) | about 25x (processing takes about 9 ms) |
| Moshi, leaf signature edit (step 42, two batches) | 212 | 40 | 5.3x (4.8–5.8x) | |
| Moshi, leaf signature edit (step 43, slower machine) | 186 | 40 | 4.7x | step 43 cut 8 ms (A/B on one machine) |
| Moshi, leaf body edit (step 44, slower machine) | 216 | 25 | 8.6x | |
| Moshi, leaf signature edit (step 44, slower machine) | 217 | 35 | 6.2x | steps 43–44 cut 10 ms (A/B on one machine) |
| Moshi, leaf body edit (step 45, slower machine) | 232 | 26 | 8.9x | |
| Moshi, leaf signature edit (step 45, slower machine) | 225 | 33 | 6.8x | step 45 cut 5–6 ms (A/B against step 44's libkc) |
| Moshi, leaf body edit (step 46, Gradle's layout) | 200 | 22 | 9.1x | the layout cut stock's teardown by about 15 ms |
| Moshi, leaf signature edit (step 46, Gradle's layout) | 207 | 28 | 7.4x | |
| Moshi, leaf body edit (step 47) | 272 | 20 | 13.6x (10.0x against step 46's stock) | stock's teardown varies from batch to batch (116 ms here, 30 in step 46) |
| Moshi, leaf signature edit (step 47) | 274 | 24 | 11.4x (8.6x against step 46's stock) | step 47 cut 1–2 ms (A/B) |
| Moshi, leaf body edit (step 48) | 218 | 18 | 12.1x (11.1x against step 46's stock) | step 48 cut 2–3 ms (A/B, 18 against 21) |
| Moshi, leaf signature edit (step 48) | 219 | 22 | 10.0x (9.4x against step 46's stock) | |
| Moshi, leaf body edit (step 49) | 209 | 21 | 10.0x | |
| Moshi, leaf signature edit (step 49) | 211 | 23 | 9.2x | |
| Moshi, leaf body edit (step 50) | 184 | 16 | 11.5x | the faster machine: kc 16 against 21 ms, stock's process phase 96 against 108 |
| Moshi, leaf signature edit (step 50) | 194 | 20 | 9.7x | step 50 cut 0.7 ms off the two updates (A/B, 5.4 against 6.1 ms) |
| Moshi, leaf body edit (step 51) | 359 | 17 | 21x (10.8x against step 50's stock) | stock's teardown took 204 ms here, 30–116 before |
| Moshi, leaf signature edit (step 51) | 340 | 21 | 16x (9.2x against step 50's stock) | step 51 cut 0.3–0.4 ms (A/B), inside the totals' spread |
| Moshi, leaf body edit (step 52) | 227 | 18 | 12.6x | stock's teardown 72–75 ms, 192–204 in step 51 |
| Moshi, leaf signature edit (step 52) | 223 | 19 | 11.7x (10.2x against step 50's stock) | step 52 cut 2 ms (A/B, 20 against 22; round 2's update 1.0 against 2.7 ms) |
| Moshi, leaf body edit (step 54) | 199 | 21 | 9.5x (10.8x against step 52's stock) | stock's teardown 42 ms, 72–75 in step 52 |
| Moshi, leaf signature edit (step 54) | 205 | 21 | 9.8x (10.6x against step 52's stock) | step 54 doesn't apply to Moshi (patch 0017 skips its updates) |
| Moshi, leaf body edit (step 55) | 203 | 18 | 11.3x | |
| Moshi, leaf signature edit (step 55) | 197 | 19 | 10.4x | step 55 cut about 2 ms (A/B, equal environments, 20 against 23 and 21 against 23) |
| Moshi, leaf body edit (step 57) | 297 | 21 | 14.1x (9.7x against step 55's stock) | stock's teardown 139 ms, 42–75 in steps 52–54 |
| Moshi, leaf signature edit (step 57) | 315 | 22 | 14.3x (9.0x against step 55's stock) | step 57 cut round 1's read 0.5 ms (A/B, equal environments) |
| Moshi, leaf body edit (step 58) | 209 | 17 | 12.3x (11.9x against step 55's stock) | |
| Moshi, leaf signature edit (step 58) | 219 | 17 | 12.9x (11.6x against step 55's stock) | step 58 doesn't apply to Moshi (no class calls in warm runs) |
| Moshi, one-file test, warm | 204 | 23 | 8.9x | |

Hilt's leaf body edit is the first edit at 10x. Since step 37, a body
edit that declares no locals runs no check, and since step 38 an update
reads only the files that changed: each of a body-edit run's two Go
updates takes about 1 ms. A Moshi body-edit run is now about
21 ms (step 42): the processors about 9, KSP's incremental bookkeeping about 5,
kc-aa's rounds about 4, libkc about 2.5, and the rest IntelliJ core and
KSP's symbol layer. What is left:

- **Signature edits, and body edits that declare locals, still cost
  more than body edits.** Since step 39 they check only the files that
  look up the declarations that changed. Since step 43 they resolve the
  signatures only of the files those reach by name (2 of 100 on Moshi
  and Hilt), and since step 44 the files they can't reach are stubs: a
  path and a package, not a copy of their declarations. Together that
  took 10 ms off a Moshi signature run and 20 off a Hilt one, measured
  on one machine. Since step 45 the files a changed declaration reaches
  are found through each model's hashed keys, not by scanning every
  file's lookups, which took another 5–6 ms off. Each of their two
  updates now takes about 3 ms in the JVM (`native`), against about 1
  for a body edit's. Hilt's signature edit is at 10x and Moshi's at
  6.8x. What is left of a Moshi update is spread out: the check (about
  1 ms), lowering the edited file, reading, parsing and the model.
  Since step 49 the index `reachable` builds over every file's
  declarations is kept between updates and patched for the files whose
  parses changed, which took 0.2–0.5 ms off a signature run's two
  updates (A/B, 7.9 against 8.3 ms on Hilt); the run totals didn't move
  outside the batches' spread. Since step 50 an update whose
  check covers at most 8 files lowers and checks them on the calling
  goroutine, and parses only the files left to parse: waking Go's parked
  threads cost more than they saved. That took 0.4–0.7 ms off a
  signature run's two updates (A/B, 5.4 against 6.1 ms on Moshi);
  `KC_ALWAYS_PARALLEL=1` restores the fan-outs. Since step 52 a KSP
  run's second update, which compared the edited file with the last
  run's round 2 and checked it again, takes its model from round 1's
  check when nothing the generated files declare (Kotlin or Java) can
  reach it: round 2's update drops to a body run's (1.0 against 2.7 ms
  on Moshi, 1.8 against 3.2 on Hilt), 2 ms off a signature run on both
  (A/B); `KC_NO_BORROW=1` turns it off. A signature run now costs
  about 1 ms more than a body run. Step 43's session ran on a slower machine: kc's unchanged body path
  took 26 ms on Moshi there, against 20.5 in step 42's, so its rows
  aren't comparable with step 42's.
- **KSP's incremental bookkeeping** (about 3 ms on Moshi since fork
  patch 0014 stat-s each output once, step 41): updating the
  lookup caches (flushed every run), checking every output against its
  backup (a `stat` of each), collecting the symbols of the edited and
  generated files, and `calcDirtyFiles`' dirtiness propagation. Stock
  pays all of it; trimming it in the fork is KSP-side. Until step 46 the
  bench's layout sent each backup path through `toRelativeString`'s
  slow path, which Gradle's layout doesn't. Since step 47 (fork patch
  0016) the check of clean outputs against their backups runs on another
  thread while the processors run, which took 1.5–3 ms off the
  teardown. Since step 48 (patch 0017) a lookup-cache update that would
  leave the storage as it is, because the dirty files look up what the
  storage already holds for them, is skipped along with its flush. It
  fires on every Moshi run and on Hilt's body runs, and took 2–3 ms off
  Moshi's body run. What is left is `calcDirtyFiles` (about 1.1 ms,
  `DirtinessPropagator` among it), the caches' up-to-date marker,
  deleted and created again each run, and the wait for the output
  writers (about 0.4). Step 53 profiled Hilt: its largest piece is
  `registerGeneratedFiles`, about 1 ms a run collecting the symbols of
  the generated Java files, then the lookup storages' update and flush
  on signature runs (about 0.9 ms more than a body run's). Reusing the
  symbols of generated files whose text was unchanged (fork patch 0018)
  made Hilt 2–3 ms slower (A/B): the processors' first
  `getSymbolsWithAnnotation` then built the same declarations, and the
  processors ran slower beside patch 0016's backup check, which the
  visit had left room for. It was reverted. Step 54 found that A/B
  biased by the environment (below), and step 55 re-ran it with the
  switch set in both arms: the patch, now 0019, takes 1–2 ms off
  `between` on Hilt and about 2 ms off Moshi's runs, and `process`
  doesn't grow, so it is back. `KSP_NO_SYMBOL_REUSE=1` turns it off. Since
  step 54 (patch 0018) an update the skip leaves to run, as on Hilt's
  signature edits, writes only the records that changed instead of
  every record the tracker has, and the tracker's paths are made
  relative once per process: 0.6–0.9 ms off Hilt's teardown (A/B).
  `KSP_NO_LOOKUP_DELTA=1` turns it off. In KSP's layout, the full
  update's removal of the dirty files' old records drops nothing (the
  records are under absolute paths, the dirty files relative ones), in
  stock too.
- **kc-aa's rounds**: about 5 ms per Moshi body run since step 46. That
  step made the JVM keep unchanged files' PSI by libkc's text hash
  (no stat or read per file per round), cached the session's source
  listings by directory time, and shared the edited file's parse
  between a run's two updates: 1.7 ms off. What is left is the two
  updates (about 1.3 ms each), IntelliJ parsing the edited file, and the
  second round's shift of its model. Since step 46 the bench passes
  Gradle's layout (one project directory over the sources and the
  outputs), which also sped up stock: its Moshi rows from step 46 on
  aren't comparable with earlier ones. Since step 50 `KC_TIMING`
  splits a round's `build`: joining kc's trees with IntelliJ's takes
  0.7–0.9 ms in round 1 (IntelliJ parsing the edited file, `joined=1`)
  and 0.2–0.3 ms in round 2, which joins nothing; why is open. Building
  the files' symbols takes about 0.2 ms per round. Since step 51
  the session makes and parses the PSI of the files Gradle says were
  modified on another thread while libkc updates. Round 1's join drops
  to about 0.1 ms, but libkc's update is slower beside the parse, and
  body runs sometimes wait for it. That took 0.3–0.4 ms off a Moshi run
  and nothing off a Hilt one (A/B); `KC_NO_PARSE_AHEAD=1` turns it off.
  Since step 59 it also parses every Kotlin file no round joined yet:
  all of them in a fresh JVM's first round, and the generated ones in
  the rounds after. That takes 350 ms off a cold Moshi run, 190 off
  Hilt's and 760 off Room's. `KC_NO_AHEAD_NEW=1` restricts it to the
  modified files again.
  Since step 56 a run's second round takes its first round's texts
  without statting the files (kc-aa opens the session with
  `FixedSources`), and libkc keeps the last two updates' Java parses
  by text: round 2's read drops by 0.05–0.16 ms and Hilt's Java parse
  (0.3–0.4 ms) is gone (A/B, too small to show in the totals);
  `KC_NO_ROUND_REUSE=1` turns both off.
- **Comment and KDoc edits outside bodies** still take the partial
  check, though they would only need nodes and positions moved, as
  step 37 does for bodies.
- **Run-to-run spread**: in one JVM, kc's body-edit runs range from
  about 21 to 45 ms even after 70 runs, and the slow runs are slower in
  every phase, the processors' too. Since step 54 the A/Bs set their
  switch in both arms (`=0` and `=1`): setting an environment variable in
  one arm only shifted Hilt's runs by about 2 ms, in phases the change
  under test doesn't touch. Until step 57, a run's first libkc update
  stalled about 1 ms in its read in a third of the runs: it statted the
  sources in goroutines while the JVM's threads were busy. It now stats
  them on the calling goroutine (`KC_NO_SERIAL_STAT=1` turns that off),
  and no run's read reaches 1 ms.

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
  calls for classes it has seen. Since step 58 the Java sources'
  classes are kept too, by the id libkc gives the library it keeps
  while the Java sources are the same (`Model.JavaID`), and so are the
  names the class path has no class of: a warm Hilt run makes no
  `KcOps.cls` call (`KC_NO_JAVA_DECLS=1` turns that off).
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

Step 59 (Room, Hilt and Moshi, 100-file modules, 4 CPUs, JDK 21) parses
the files no round has joined yet while libkc updates (A/B in
`analysis-api.md`). Cold run medians of 5 stock JVMs and 15 kc JVMs (6
for Room):

| ms | stock KSP2 | kc | ratio |
|---|---|---|---|
| Moshi, cold run (step 59) | 11411 | 5677 | 2.0x |
| Hilt, cold run (step 59) | 19570 | 8593 | 2.3x |
| Room, cold run (step 59) | 24818 | 11860 | 2.1x |

Kc's own cold work left is:

- the environment, about 0.5–0.6 s;
- the first update, about 0.2–0.45 s;
- round 2's native update, slower beside the parse;
- Hilt's serial Java PSI, about 0.5 s.

The rest is warm-up that both backends share.

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
