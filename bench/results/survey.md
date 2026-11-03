# Survey of 9 Kotlin libraries

Written by `bench/survey -repos` (see `bench/survey.sh`). For each repository that `bench/survey/repos.txt` lists, at its pinned release tag, a detector that reads kc's tokens lists the features each file uses that kc doesn't support. A file is *blocked* by each of them. The ranking counts the library code that the JVM compilation includes (the `main` sets, 1128 files); the tests (1373 files) are counted apart. The detector only sees syntax and the names of the 147 inline-only functions of the standard library's default imports, so an unblocked file isn't guaranteed to compile. Each repository's own report, with every file's blockers, is in `bench/results/survey/`.

## Repositories

| Repository | Tag | Files | Lines | Unblocked | Median blockers left | Test files | Tests unblocked |
|:---|:---|---:|---:|---:|---:|---:|---:|
| kotlinpoet | 2.4.0 | 39 | 8696 | 39 | – | 37 | 37 |
| okio | parent-3.18.2 | 96 | 17346 | 96 | – | 99 | 99 |
| kotlinx-serialization | v1.11.0 | 99 | 16291 | 99 | – | 212 | 212 |
| kotlinx-coroutines | 1.11.0 | 169 | 34248 | 169 | – | 456 | 456 |
| kotlinx-datetime | v0.8.0 | 69 | 13870 | 69 | – | 59 | 59 |
| exposed | 1.5.1 | 169 | 32485 | 169 | – | 124 | 124 |
| ktor | 3.6.0 | 236 | 23774 | 236 | – | 78 | 78 |
| arrow | 2.2.3 | 107 | 26909 | 107 | – | 191 | 191 |
| mockk | v1.14.11 | 144 | 17302 | 144 | – | 117 | 117 |
| **all** | | 1128 | 190921 | 1128 | – | 1373 | 1373 |

## Library code

### Features left, by files blocked

1128 of 1128 files use no feature that kc doesn't support. *Repositories* is the number of repositories with a file the feature blocks, and *mean share* the mean, over all 9 repositories, of the share of each one's files that it blocks, so that a large repository doesn't decide the order alone. *Only blocker* counts the files it is the last blocker of.

| Feature | Files | Repositories | Mean share | Only blocker | Uses |
|:---|---:|---:|---:|---:|---:|

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files. *Repositories gaining* counts the repositories with a file it unblocks.

| Step | Feature | Files unblocked | Repositories gaining |
|---:|:---|---:|---:|

### Top blockers by repository

The features that block the most of each repository's library files, with the number of files.

| Repository | Blocked | Top blockers |
|:---|---:|:---|
| kotlinpoet | 0 |  |
| okio | 0 |  |
| kotlinx-serialization | 0 |  |
| kotlinx-coroutines | 0 |  |
| kotlinx-datetime | 0 |  |
| exposed | 0 |  |
| ktor | 0 |  |
| arrow | 0 |  |
| mockk | 0 |  |

## Tests

### Features left, by files blocked

1373 of 1373 files use no feature that kc doesn't support. *Repositories* is the number of repositories with a file the feature blocks, and *mean share* the mean, over all 9 repositories, of the share of each one's files that it blocks, so that a large repository doesn't decide the order alone. *Only blocker* counts the files it is the last blocker of.

| Feature | Files | Repositories | Mean share | Only blocker | Uses |
|:---|---:|---:|---:|---:|---:|

### Greedy order of the features left

At each step, the feature that unblocks the most files given the ones before it, or, if none unblocks a file, the one that blocks the most files. *Repositories gaining* counts the repositories with a file it unblocks.

| Step | Feature | Files unblocked | Repositories gaining |
|---:|:---|---:|---:|

## Cross-check with the parser

The parser, recovering after each declaration that fails, reported 0 errors in all the files, library and tests. 0 of them name a feature that the detector also found in the file.

## Cross-check with the checker

kc's checker, run on each repository's library files together with only the standard library and `kotlin-reflect` on the class path, reports these unsupported features. Without the repositories' dependencies it can't resolve many names, and it skips the declarations the parser rejects, so it misses features; but where it reports one, the code has it. *Detected* counts the files where the detector found the feature too. A feature the detector misses isn't in the ranking above.

| Feature | Files | Detected | Repositories | First |
|:---|---:|---:|---:|:---|

