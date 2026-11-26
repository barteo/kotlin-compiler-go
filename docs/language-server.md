# kc-lsp for Kotlin/JVM

Build the server from the kc repository and configure your editor's Kotlin language client to launch it over stdio.

```sh
CGO_ENABLED=0 go build -o bin/kc-lsp ./cmd/kc-lsp
bin/kc-lsp --version
```

Use the absolute path to `bin/kc-lsp` as the command, `["--stdio"]` as its arguments, and `kotlin` as the document language ID. Give the client the project root as its workspace folder. The release packaging workflow includes the server beside `kc`.

kc-lsp parses and checks Kotlin inside the Go process. It reads installed JDK classes, stdlib metadata, JARs and Java sources as files. Set `JAVA_HOME` to the JDK home for JDK symbols and `KOTLIN_HOME` to a Kotlin distribution for standalone projects that use the standard library. A basic Kotlin document can be edited with neither installed.

## Opening a project

A folder with no build file becomes one source module. The Maven importer reads reactor modules, parent POMs, dependency management, imported BOMs, exclusions and conventional production and test roots. The Gradle importer handles literal project includes, conventional JVM source sets, project dependencies and version catalog library and plugin aliases. It tracks the difference between a project's own compile dependencies and the dependencies it exports through `api` and `compileOnlyApi`. A supported Kotlin JVM plugin declaration supplies the default standard library version. The importer honors `kotlin.stdlib.default.dependency=false` for that dependency.

The server checks local Maven and Gradle caches first. It can fetch versioned POMs, JARs and optional source JARs over HTTPS from Maven Central. Maven uses Central by default. For Gradle, declare `mavenCentral()` in the project repositories or in settings under `dependencyResolutionManagement`.

| Environment variable | Effect |
| --- | --- |
| `MAVEN_REPO_LOCAL`, `GRADLE_USER_HOME` | Override the local Maven repository and Gradle user home, respectively. |
| `KC_LSP_CACHE_DIR` | Store downloads in the `maven` subdirectory. The default is the operating system's user cache under `kc-lsp`. Download caches are separate for each repository URL. |
| `KC_LSP_MAVEN_REPOSITORY` | Use the supplied HTTPS repository base URL for missing Maven artifacts. |
| `KC_LSP_OFFLINE=1` | Read cached artifacts only. Import reports the coordinates of missing dependencies. |

For a build that computes its source sets or dependency graph in plugin code, put a resolved `kc-lsp.json` at the workspace root. It takes precedence over build discovery. Paths are relative to this file unless absolute. Each module is one Kotlin compilation. List both production and test compilations if their classpaths differ.

```json
{
  "version": 1,
  "modules": [
    {"name": "core", "sourceRoots": ["core/src/main/kotlin"]},
    {
      "name": "app",
      "sourceRoots": ["app/src/main/kotlin"],
      "javaRoots": ["app/src/main/java"],
      "generatedRoots": ["app/build/generated/ksp/main/kotlin"],
      "dependencies": ["core"],
      "classPath": ["libs/kotlin-stdlib.jar"],
      "sources": ["libs/kotlin-stdlib-sources.jar"]
    },
    {
      "name": "app-test",
      "sourceRoots": ["app/src/test/kotlin"],
      "dependencies": ["app"],
      "friendModules": ["app"]
    }
  ]
}
```

`classPath` is ordered. `sources` holds optional source archives. For a binary friend dependency, include its entry in `classPath` and `friendPaths`. Each module can set `jdkHome`, `explicitAPI` and `languageVersion` (currently `2.4`). Build import errors appear in the editor. Clients supporting file watch registration can reload after build, source or dependency changes. Restart the server after external changes in clients that omit watch support.

## Behavior and parity

The comparison target is [JetBrains kotlin-lsp v271.2085.0](https://github.com/Kotlin/kotlin-lsp/releases/tag/kotlin-lsp/v271.2085.0). This is a preview for Kotlin/JVM.

| Operation | Implemented behavior | Remaining differences |
| --- | --- | --- |
| Symbols, folding, hover and diagnostics | The standalone fixture matches the upstream outline, fold ranges, definition, KDoc hover and diagnostics. The server publishes versioned diagnostics after edits and supports pull diagnostics. | Diagnostics follow the kc parser and checker. IntelliJ inspections report additional findings. |
| Completion and signature help | Completion ranks locals, implicit class or lambda members, imported declarations and source auto-imports. Items carry standard text edits and support snippets, insert/replace ranges and source KDoc on resolve. Signature help covers partial source and compiled Kotlin calls, constructors, named and default parameters, and varargs. | IntelliJ templates, DSL-specific scope rules and ranking for complex generic overloads have wider coverage upstream. |
| References, highlights and rename | Cross-module references, named arguments, import aliases and source method override families. Rename returns versioned workspace edits when the client supports them. | Rename declines possible collisions and incomplete coverage, including read-only overrides. General Java refactorings remain open. |
| Navigation and hierarchies | Go to Definition returns the origin and target identifier ranges when the client enables definition links. For overloaded Java methods it matches the compiler’s JVM descriptor against workspace or attached source. For compiled Kotlin functions it matches parameter and extension receiver types against attached sources, including imported aliases. The server also supports call and type hierarchies, implementations, and workspace symbols from source archives. | A missing source attachment, an unresolved signature or multiple matching declarations can leave a library call without a destination. |
| Semantic tokens and inlay hints | Full and range token requests use the upstream legend. The server classifies declarations, named arguments, labels (including the adjoining `@`), documentation and Kotlin modifiers. Hints include inferred declaration types, expression-body return types, argument names, implicit lambda parameters and receivers, lambda result expressions and intermediate types in multiline call chains. | Modifier classification has partial coverage. Context-parameter hints and clickable hint labels remain open. |
| Formatting and imports | Document and range formatting for multiline calls, parameter lists, expression bodies, call chains and lambdas. The server also sorts and removes imports. | IntelliJ covers additional wrapping, code-style and import rules. |
| File and package moves | Moving Kotlin files together or renaming a package folder updates directory-matching package declarations, imports and qualified references in the module and its Kotlin consumers. Open documents keep their unsaved text at their new URIs. | Package changes must stay within one module and use named packages. The server declines moves that need Java reference coverage in the module or its consumers, split an explicit overload import, or introduce binding conflicts. |
| Code actions | Import a visible source or compiled Kotlin name, simplify a redundant safe call, non-null assertion or Elvis fallback, remove an extra positional annotation argument or an invalid `inner` modifier, suppress a warning on a statement, function or class, and extract or inline a local literal. | Additional Kotlin and kotlinx intentions and inspections remain open. |
| Project import | Static Maven and Gradle Kotlin/JVM project models, remote artifacts and an explicit resolved model. | Dynamic plugins and variants need an explicit model. Older Kotlin language modes, Multiplatform and Android are outside this preview. |

The editor model resolves unsaved source dependencies and rechecks the changed module and its consumers. Full overload selection across module visibility boundaries and reference searches originating in Java need further coverage.

Clients can send dotted or nested `jetbrains.kotlin.hints` settings in `initializationOptions` or `workspace/didChangeConfiguration`. Supported boolean keys under that prefix are `settings.types.property`, `settings.types.variable`, `type.function.return`, `type.function.parameter`, `parameters`, `parameters.compiled`, `settings.lambda.return` and `lambda.receivers.parameters`. Each defaults to true. Enable `call.chains` to show intermediate receiver types when a multiline chain has at least two distinct types to display. It defaults to false.

## Verification

```sh
go test -race ./internal/lsp
go test ./internal/lsp -run '^$' -bench BenchmarkWorkspace -benchmem
```

The offline stdio tests replay a standalone file, a two-file Maven project and a Gradle project with a separate library module. Imported-project captures wait for navigation from the consumer to the other file before recording results. Tests compare definition and hover payloads, reference locations, and the final text of every renamed file. [Standalone capture](../internal/lsp/testdata/upstream-271.2085.0.json) · [Maven capture](../internal/lsp/testdata/maven/upstream-271.2085.0.json) · [Gradle capture](../internal/lsp/testdata/gradle/upstream-271.2085.0.json)

To regenerate a capture, run `python3 tools/lsp/capture.py /path/to/bin/intellij-server --output baseline.json` with the pinned distribution. Add `--maven` or `--gradle` for the corresponding project. Regeneration uses the upstream JVM server and its build tooling. Running kc-lsp and replaying the checked-in results use Go.

The editing fixture puts the consumer and receiver declarations in separate modules and adds a compiled Kotlin library. JetBrains imports the Gradle project. The offline Go replay loads the equivalent resolved module graph. It checks the relative order of three completion candidates and the actual inserted text, both compiled overload signatures and their active parameters, and six quick fixes. It also compares a cross-module definition link with a supplementary Unicode character before the reference, plus full and range semantic tokens for loop, lambda and receiver labels. For each fix it compares the diagnostic under the cursor, the edited document and the diagnostics after the edit. The annotation cases cover sole and trailing arguments and verify that an unknown named argument has no removal action. The sole-argument fixture suppresses the separate IntelliJ inspection for empty annotation parentheses. Regenerate it with `--editing --kc /path/to/kc`. [Recorded editing results](../internal/lsp/testdata/editing/upstream-271.2085.0.json)

The navigation replay compares six calls against a Gradle project with Java and Kotlin sources. The Go replay compiles the Kotlin library, attaches its sources and checks the same target files and UTF-16 ranges. It exercises Java both as workspace source and as a binary with attached source. The hint replay compares visible labels, positions, kinds and padding with default and toggled settings, including a partial-range request after an unsaved edit. The capture supplies the pinned provider’s `hints.lambda.return` option for the extension’s `hints.settings.lambda.return` setting. Regenerate these results with `--navigation`. [Recorded navigation and hint results](../internal/lsp/testdata/navigation/upstream-271.2085.0.json)

The formatting replay compares the resulting Kotlin text for nested calls, chained selectors, lambdas, and a selection inside a call. A range uses the indentation already present around it. The four-space fixtures match the pinned JetBrains defaults. `kc-lsp` also honors the client’s `tabSize` and `insertSpaces` options for two-space and tab-indented projects.

The move replay compares the final text for a nested package folder rename and a two-file move with an aliased import in a dependent module. It applies the returned edits, renames the files, sends `workspace/didRenameFiles`, then checks diagnostics and navigation at the new paths. Regenerate the reference with `--moves`. [Recorded move results](../internal/lsp/testdata/moves/upstream-271.2085.0.json)

The benchmark generates Kotlin-only workspaces and measures import plus indexing, warm requests and an edit through consumer diagnostics. One Apple M2 run, three iterations per operation, produced these timings.

| Workspace | Cold index | Warm hover | Warm completion | Edit to diagnostics |
| --- | ---: | ---: | ---: | ---: |
| 8,000 lines, one module | 64 ms | 12 µs | 0.66 ms | 44 ms |
| 40,000 lines, four modules | 342 ms | 4.9 µs | 5.8 ms | 268 ms |

For the editing change, a separate Apple M2 comparison measured the same warm member-completion and edit-to-diagnostics cases on the original branch commit (`1697f3d`) and with the receiver index. Values are medians of three runs of five iterations. The larger fixture's diagnostics time still reflects a module-wide recheck.

| Workspace | Member completion before → after | Edit to diagnostics before → after |
| --- | ---: | ---: |
| 8,000 lines | 0.500 → 0.046 ms | 44.4 → 45.0 ms |
| 40,000 lines | 5.09 → 0.063 ms | 263 → 269 ms |
