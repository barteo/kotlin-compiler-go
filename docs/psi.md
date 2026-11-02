# PSI and analysis

Two public packages give tools such as linters a JVM-free view of Kotlin
code: [`psi`](../psi/doc.go) parses it, and [`analysis`](../analysis/analysis.go)
runs `kc`'s checker on what `psi` parsed and reports what it resolved.

```
.kt text ─▶ psi.Parse ─▶ *psi.Snapshot ─▶ internal/lower ─▶ kc AST + provenance
                                                   ─▶ check (with a Recorder) ─▶ analysis facts
```

## psi

`psi.Parse(path, text)` never fails: syntax errors are diagnostics on the
snapshot, and the tree stays lossless (its leaves' texts concatenate to the
source). The parser and lexer are ports of Kotlin 2.0.21's, so element
kinds, nesting, whitespace and comment placement match the Kotlin
compiler's PSI. The tests compare trees with dumps from stock Kotlin made by
[`tools/psidump`](../tools/psidump/README.md) and committed under
`psi/testdata`, so CI needs no JVM.

- **Nodes** live in one preorder array. IDs run from 1 (the `FILE` node) to
  `NodeCount()`, a parent's ID is lower than its children's, and a node's
  subtree is exactly `[ID, SubtreeEnd)`. A `NodeRef` is a small comparable
  value; the zero `NodeRef` is invalid and every method is safe on it.
- **Kinds** are numbered by `psi/kinds.txt`, which is append-only, so kind
  numbers can be persisted. `go generate ./psi` regenerates
  `kinds_gen.go`; CI runs `go run ./cmd/psi-kinds -check`.
- **Offsets** are UTF-8 byte offsets. A snapshot converts them to UTF-16
  offsets and to 1-based lines and columns. `psi` doesn't normalize line
  endings.
- **Snapshots** are immutable and safe for concurrent use. A `Parser` can
  be reused per worker; snapshots don't keep its buffers.

## analysis

`analysis.AnalyzeArena(ctx, cfg, snapshots)` analyzes the snapshots as one
module:

1. Each snapshot is lowered to `kc`'s AST by `internal/lower`, directly
   from the tree, with a provenance table from AST node to psi node. The
   lowering produces the same AST as `kc`'s own parser; a differential test
   compares them on every `testdata/**/*.kt`, and CI runs the whole test
   suite with `KC_FRONTEND=psi`.
2. The library (JDK, class path) is opened as `kc` opens it, and the files
   are checked by `check.CheckContext`, which honors cancellation. A
   `check.Recorder` reports what the AST doesn't keep: when coverage,
   whether a statement's value is used, unreachable statements.
3. The checked AST is walked back to psi nodes, and facts are exported
   keyed by `ElementID{Snapshot, Node}`.

Code that `kc` can't compile is reported as diagnostics (`source_error`,
or `unsupported:<feature>` for constructs `kc` doesn't support) and no
facts at all: callers must not act on partial results. Scripts (`.kts`)
return `ErrUnsupported`.

`Data.Capabilities` lists the fact families that are computed; a fact field
of a family outside it is always zero, and a caller that needs it should
refuse to run rather than read zeros. `Result`'s methods return deep copies.

### Where facts come from

| Fact | Source |
|---|---|
| `Type` | the checked expression's or declaration's `types.Type`, converted to Kotlin's names (`kotlin.Any`, `kotlin.Function1`, ...); an `if`, `when` or `try` used as a statement has the type the recorder reports for it |
| `Symbol` | the symbol an identifier, member, call or declaration resolved to, interned per result; `when (val x = e)`'s `PROPERTY` declares `x` |
| `Call` | the resolved function, argument mapping, receivers, type arguments and inline flags; on a compound assignment and `x++`, the operator the checker rewrote it to; on a callable reference, the function or constructor it names; `suspend { }` is a call of `kotlin.suspend`; a member function the checker builds in, `1.plus(2)` or `"a".plus(b)`, is named by its receiver's type, `kotlin.Int.plus`, except those the standard library declares as extensions: `x.toString()` on a nullable `x` is `kotlin.toString` and `s.isEmpty()`, `uppercase`, `lowercase`, `substring` and `toCharArray` are `kotlin.text.*`, with `Extension` and `DeclaredReceiver`; a function-type value's call `f()` has no stable parameter names, and its callee reference the property's symbol; a delegate `by lazy { e }`'s `getValue` returns `e`'s type |
| `When` | the recorder; `Exhaustive` and `Missing` ignore the `else` branch |
| `FlowNonNull`, `DefinitelyNonNull` | a nullable declaration read as non-null after a smart cast; a safe access `x?.a` on an `x` that can't be null, whose `Type` stays nullable, as kotlinc's |
| `UsedAsExpression`, `Unreachable` | the recorder; an Elvis whose left side can't be null is unreachable, as kotlinc reports it |
| `SuspendOperation` | a `for` loop whose `iterator()`, `hasNext()` or `next()` is a suspend operator |
| `ReturnTarget`, `ReceiverTarget` | the jump's lambda or enclosing function; `this`'s receiver |
| `ImplicitReceiver`, `Call.ImplicitReceiver` | on a call (and its callee reference) or property reference with no explicit receiver: the lambda with a receiver, extension function, or class or object whose `this` supplies it |
| `Type.SAM` | a `fun interface`, or a Java interface with one abstract method that Kotlin doesn't map to its own type (`Runnable`, not `Comparable`) |
| `Call.Annotations`, `OwnerAnnotations`, `PackageAnnotations` | the callee declaration's annotations, or a library method's class-file annotations; its class's (not an enclosing class's); a library class's package's `package-info` |
| `Deprecated` | the `@kotlin.Deprecated` message, folded as a constant, of what a reference or `USER_TYPE` names (a constructor falls back to its class), or `Deprecated in Java`; set only for the `WARNING` level |
| `Symbol.Shadowed` | a local, `for` variable, local delegate, explicit lambda parameter or local function parameter whose name an enclosing local or parameter already has (`_` and the implicit `it` never shadow, as kotlinc's `NAME_SHADOWING`) |
| `Call.Targets` | the functions the callee overrides, from the checker's own override resolution, each with its own `Targets`: the tree kotlinc's `overriddenTreeUniqueAsSequence` walks |
| `ImpossibleCast` | an `as` between types kotlinc's `isCastPossible` rejects: unrelated, with either side final; a `null` cast to a non-null type |
| `Accessors` | on a property access, the accessors it can call: a Java getter and its setter (`java.util.Calendar.getFirstDayOfWeek`, `setFirstDayOfWeek`), or a Kotlin property's `<get-p>` and, for a `var`, `<set-p>`, with the overridden properties' accessors as `Targets`; not a `Call`, which rules read as "this is a call" |
| `Call` on `==`, `!=` | the `equals` found first up the left operand's supertypes, with `kotlin.Any.equals` as a target |
| `Package` | on each name of a package that qualifies a reference to a top-level function, property, class or object, its fully qualified name: `kotlin` and `kotlin.math` in `kotlin.math.floor(x)`, `java.lang` in `java.lang.Math.max(a, b)` |
| `Symbol.HasAbstractMembers`, `HasConcreteMembers` | whether a class's member scope, as kotlinc's `unsubstitutedMemberScope`, holds an abstract member or one that isn't: the class's and its supertypes' members that nothing in the hierarchy overrides, `kotlin.Any`'s open members and the nested classes |

Facts recorded on an expression are copied to the wrapper nodes Kotlin's
PSI puts around it (`DOT_QUALIFIED_EXPRESSION`, `PARENTHESIZED`, labels,
annotations), and a type reference's type is set on both its `USER_TYPE`
and its `TYPE_REFERENCE`.
