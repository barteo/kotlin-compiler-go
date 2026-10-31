# Survey of 9 Kotlin libraries

Written by `bench/survey -repos` (see `bench/survey.sh`). For each repository that `bench/survey/repos.txt` lists, at its pinned release tag, a detector that reads kc's tokens lists the features each file uses that kc doesn't support. A file is *blocked* by each of them. The ranking counts the library code that the JVM compilation includes (the `main` sets, 1128 files); the tests (1373 files) are counted apart. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile. Each repository's own report, with every file's blockers, is in `bench/results/survey/`.

## Repositories

| Repository | Tag | Files | Lines | Unblocked | Median blockers left | Test files | Tests unblocked |
|:---|:---|---:|---:|---:|---:|---:|---:|
| kotlinpoet | 2.4.0 | 39 | 8696 | 39 | – | 37 | 37 |
| okio | parent-3.18.2 | 96 | 17346 | 95 | 1 | 99 | 99 |
| kotlinx-serialization | v1.11.0 | 99 | 16291 | 98 | 1 | 212 | 212 |
| kotlinx-coroutines | 1.11.0 | 169 | 34248 | 169 | – | 456 | 455 |
| kotlinx-datetime | v0.8.0 | 69 | 13870 | 69 | – | 59 | 59 |
| exposed | 1.5.1 | 169 | 32485 | 168 | 1 | 124 | 124 |
| ktor | 3.6.0 | 236 | 23774 | 236 | – | 78 | 78 |
| arrow | 2.2.3 | 107 | 26909 | 99 | 1 | 191 | 190 |
| mockk | v1.14.11 | 144 | 17302 | 143 | 1 | 117 | 117 |
| **all** | | 1128 | 190921 | 1116 | 1 | 1373 | 1371 |

## Library code

### Features left, by files blocked

1116 of 1128 files use no feature that kc doesn't support. *Repositories* is the number of repositories with a file the feature blocks, and *mean share* the mean, over all 9 repositories, of the share of each one's files that it blocks, so that a large repository doesn't decide the order alone. *Only blocker* counts the files it is the last blocker of.

| Feature | Files | Repositories | Mean share | Only blocker | Uses |
|:---|---:|---:|---:|---:|---:|
| context parameters | 7 | 1 | 0.7% | 7 | 187 |
| member extension property | 2 | 2 | 0.2% | 2 | 7 |
| safe-call assignment | 2 | 2 | 0.1% | 2 | 2 |
| labeled expression | 1 | 1 | 0.1% | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files. *Repositories gaining* counts the repositories with a file it unblocks.

| Step | Feature | Files unblocked | Repositories gaining |
|---:|:---|---:|---:|
| 1 | context parameters | 1123 | 1 |
| 2 | member extension property | 1125 | 2 |
| 3 | safe-call assignment | 1127 | 2 |
| 4 | labeled expression | 1128 | 1 |

### Top blockers by repository

The features that block the most of each repository's library files, with the number of files.

| Repository | Blocked | Top blockers |
|:---|---:|:---|
| kotlinpoet | 0 |  |
| okio | 1 | labeled expression (1) |
| kotlinx-serialization | 1 | member extension property (1) |
| kotlinx-coroutines | 0 |  |
| kotlinx-datetime | 0 |  |
| exposed | 1 | safe-call assignment (1) |
| ktor | 0 |  |
| arrow | 8 | context parameters (7), member extension property (1) |
| mockk | 1 | safe-call assignment (1) |

## Tests

### Features left, by files blocked

1371 of 1373 files use no feature that kc doesn't support. *Repositories* is the number of repositories with a file the feature blocks, and *mean share* the mean, over all 9 repositories, of the share of each one's files that it blocks, so that a large repository doesn't decide the order alone. *Only blocker* counts the files it is the last blocker of.

| Feature | Files | Repositories | Mean share | Only blocker | Uses |
|:---|---:|---:|---:|---:|---:|
| context parameters | 1 | 1 | 0.1% | 1 | 4 |
| safe-call assignment | 1 | 1 | 0.0% | 1 | 1 |

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files. *Repositories gaining* counts the repositories with a file it unblocks.

| Step | Feature | Files unblocked | Repositories gaining |
|---:|:---|---:|---:|
| 1 | context parameters | 1372 | 1 |
| 2 | safe-call assignment | 1373 | 1 |

## Cross-check with the parser

The parser, recovering after each declaration that fails, reported 88 errors in all the files, library and tests. 88 of them name a feature that the detector also found in the file.

## Cross-check with the checker

kc's checker, run on each repository's library files together with only the standard library and `kotlin-reflect` on the class path, reports these unsupported features. Without the repositories' dependencies it can't resolve many names, and it skips the declarations the parser rejects, so it misses features; but where it reports one, the code has it. *Detected* counts the files where the detector found the feature too. A feature the detector misses isn't in the ranking above.

| Feature | Files | Detected | Repositories | First |
|:---|---:|---:|---:|:---|

