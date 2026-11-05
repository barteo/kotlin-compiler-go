# psidump

`PsiDump.java` writes the PSI tree that stock Kotlin 2.4.21 builds for a
source file. The psi package's tree-shape goldens in `psi/testdata/trees`
come from it. CI never runs it, so the Go tests need no JVM.

## Jars

Fetch these from Maven Central (`repo.maven.apache.org/maven2` or
`maven-central.storage-download.googleapis.com/maven2`) into one directory:

| Artifact | SHA-256 |
|---|---|
| `annotations-13.0.jar` | `ace2a10dc8e2d5fd34925ecac03e4988b2c0f851650c94b8cef49ba1bd111478` |
| `kotlin-compiler-embeddable-2.4.21.jar` | `ef19419c765e7ac8404465fa026ca8fc4dbb8a822de0fc79aa53c8dce29f1d02` |
| `kotlin-reflect-1.6.10.jar` | `3277ac102ae17aad10a55abec75ff5696c8d109790396434b496e75087854203` |
| `kotlin-script-runtime-2.4.21.jar` | `a658cf5902a1525a06c264a5e42b437e97a6679ddeef8ca641246f270cf6a013` |
| `kotlin-stdlib-2.4.21.jar` | `bd8250210584cb659847dce9cda660f8e9906e5def7fbebcea93dc6dcb6a88a3` |
| `kotlinx-coroutines-core-jvm-1.8.0.jar` | `9860906a1937490bf5f3b06d2f0e10ef451e65b95b269f22daf68a3d1f5065c5` |
| `trove4j-1.0.20200330.jar` | `c5fd725bffab51846bf3c77db1383c60aaaebfe1b7fe2f00d23fe1b7df0a439d` |

## Run

JDK 17 or newer runs the single-file program directly:

```bash
CP=$(ls /path/to/jars/*.jar | tr '\n' ':')
java -cp "$CP" tools/psidump/PsiDump.java psi/kinds.txt OUT_DIR [--text] [--rel ROOT] FILE...
```

It writes `OUT_DIR/<name>.tree`, where `<name>` is the file name, or the
path relative to `ROOT` when `--rel ROOT` is given, one line per node in preorder:
`<depth> <KIND> <start> <end> ["leaf text"]`. Offsets are UTF-16, as in the
IntelliJ API. An element type with no name in `psi/kinds.txt` prints as
`?debugName`.

With `--text`, nodes without children also print their text and error
elements their message (`!"message"`). The tests use `--text` dumps and
collapse KDoc's inner nodes into the single `KDOC` leaf that psi builds.

## Regenerating the goldens

From the repository root, with `CP` set as above:

```bash
(cd psi/testdata/src && java -cp "$CP" ../../../tools/psidump/PsiDump.java ../../kinds.txt ../trees --text --rel . *)
```

```bash
java -cp "$CP" tools/psidump/PsiDump.java psi/kinds.txt /tmp/corpus --text --rel . $(find testdata -name '*.kt' | sort)
```

```bash
go test ./psi -run TestCorpusTrees -update -oracle /tmp/corpus
```

Run `go test ./psi -oracle /tmp/corpus` to see the first differing line of
each corpus file that no longer matches.
