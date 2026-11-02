# psidump

`PsiDump.java` writes the PSI tree that stock Kotlin 2.0.21 builds for a
source file. The psi package's tree-shape goldens in `psi/testdata/trees`
come from it. CI never runs it, so the Go tests need no JVM.

## Jars

Fetch these from Maven Central (`repo.maven.apache.org/maven2` or
`maven-central.storage-download.googleapis.com/maven2`) into one directory:

| Artifact | SHA-256 |
|---|---|
| `annotations-13.0.jar` | `ace2a10dc8e2d5fd34925ecac03e4988b2c0f851650c94b8cef49ba1bd111478` |
| `kotlin-compiler-embeddable-2.0.21.jar` | `9fa8cdd1de0dccffe154c997d423ec6b5f53cd6d9177e3a77a9b0de03fb1bc81` |
| `kotlin-reflect-1.6.10.jar` | `3277ac102ae17aad10a55abec75ff5696c8d109790396434b496e75087854203` |
| `kotlin-script-runtime-2.0.21.jar` | `9c111f8d08ade455566272d561921adc2b2cb6b7a4ccee38d9829c5e3a1ca6a3` |
| `kotlin-stdlib-2.0.21.jar` | `f31cc53f105a7e48c093683bbd5437561d1233920513774b470805641bedbc09` |
| `kotlinx-coroutines-core-jvm-1.6.4.jar` | `c24c8bb27bb320c4a93871501a7e5e0c61607638907b197aef675513d4c820be` |
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
