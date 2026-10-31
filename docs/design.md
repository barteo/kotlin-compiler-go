# Design

```
.kt files   ─(parallel)─▶ lex + parse ─▶ declare names ─▶ resolve imports
            ─▶ resolve supertypes ─▶ resolve signatures
            ─▶ infer omitted types (sequential) ─(parallel)─▶ check bodies
            ─(parallel)─▶ generate + write .class files
.java files ─▶ read declarations (bodies skipped)
JDK, -cp    ─(concurrently with parsing)─▶ index class names
            ─(concurrently with parsing)─▶ read the default imports' metadata
            ─(on first use, during checking)─▶ read and parse class files
            ─(on first call, during checking)─▶ analyze inline functions' bodies
```

| Package | Role |
|---|---|
| `internal/syntax` | Lexer (newline-aware, as Kotlin's grammar requires), AST, recursive-descent parser |
| `internal/types` | Types and symbols, independent of the AST |
| `internal/check` | Name resolution, type checking and inference, smart casts; annotates the AST |
| `internal/classfile` | Constant pool, bytecode assembler with stack tracking and frames, class file reader and writer, instruction decoder |
| `internal/classpath` | Reads the JDK's module image (`lib/modules`), jars and class directories |
| `internal/javasrc` | Reads the declarations of a mixed module's Java sources |
| `internal/kmeta` | Decodes and encodes Kotlin metadata: `@kotlin.Metadata`, `.kotlin_module`, `.kotlin_builtins` |
| `internal/inline` | Analyzes the bytecode of library inline functions for the inliner |
| `internal/codegen` | AST to bytecode, including the inliner and the compiler plugins' code |
| `internal/driver` | Pipeline and worker pools |

Omitted return and property types are inferred first, in order, because
other files' bodies depend on them; after that the parallel phases only
read shared symbols. The class files follow `kotlinc`'s ABI; where they
differ is listed in [Unsupported Kotlin](unsupported.md#differences-from-kotlinc).

## Inputs

**Class path.** `internal/classpath` indexes class names at startup: the
JDK's `lib/modules` image (about 28,000 classes), then each `-cp` entry,
earlier entries winning. Indexing reads only the image's tables and the
jars' central directories, takes about 5 ms, and runs while the sources
are parsed. A class file is parsed when the checker first needs the
class, and then only its supertypes and member signatures.

**Java** declarations are seen as Kotlin sees them: primitives and boxes
map to `Int` and `Int?`, reference types are platform types (`String!`),
wildcards are projections, and `getX()`/`setX(v)` are properties. For a
mixed module, `.java` files on the command line are read by
`internal/javasrc`, declarations only, into the `classfile.Info` a class
file would give; `javac` compiles them later against `kc`'s output.

**Kotlin metadata.** With `kotlin-stdlib.jar` on the class path, Kotlin
declarations come from metadata, decoded by `internal/kmeta`'s small
protobuf reader: `@kotlin.Metadata` on class files,
`META-INF/*.kotlin_module` (which classes hold each package's top-level
declarations) and `.kotlin_builtins` (classes with no class files, such
as `kotlin.Int` and `List`). The collection types are the `java.util`
interfaces with members from `.kotlin_builtins`; read-only types leave
out the mutating ones.

Packages are read lazily: the first lookup in one reads its classes in
parallel, decoding only names, and a declaration is decoded when its name
is first used. The default imports (`defaultPackages`: `kotlin`,
`kotlin.collections`, `kotlin.text` and six more) are read ahead by a
goroutine once the class path is indexed, while sources are still
parsed; checking waits only for a package not read yet (`prefetch` in
`-time`). Lookups are cached per file, class members in a shared table,
and type argument lists are interned.

## Checking

**Passes.** Before bodies are checked in parallel, passes run over all
files in order:

1. Declare classes, top-level functions, properties and typealiases.
2. Resolve imports, then typealiases' right sides.
3. Resolve supertypes. This must precede any subtype check: `asSuper`
   caches results in a table shared by all files, so an early check would
   cache `false` for a supertype not yet known.
4. Find hierarchy cycles, record sealed subclasses, resolve signatures,
   give overrides their inherited default values, declare the members
   class delegation implements, and infer omitted types.

The program's classes get the same symbols as the class path's
(`types.ClassSym`), so the checker treats them alike. Member lookup walks
supertypes, superclasses first; an override hides what it overrides by
erased parameters, or for generic supertypes by substituted signature.

**Name resolution** follows Kotlin's tower of implicit receivers: members
of `this`, extensions that fit it, members of companions and objects in
scope, then top-level declarations, with explicit imports before the
package, star imports and default imports. A receiver whose functions
don't take the arguments gives way to the next. Among candidates the most
specific wins; ties prefer non-varargs, then fewer defaults, and
candidates differing only in a lambda's result type are chosen by the
body's type (`@OverloadResolutionByLambdaReturnType`). Typealiases
(`types.AliasSym`) live next to classes in their scope, and uses are
expanded, so the rest of the compiler sees only expanded types.

**Types.** `types.Type` is a small value comparable with `==`: a class
type's arguments are an interned list, and a type parameter points to its
symbol through the same field. `T & Any` is `T` with the flag `D`.

**Inference.** Each generic call gets a constraint system
(`internal/check/infer.go`) with one variable per type parameter.
Arguments add `argument <: parameter`, reduced to bounds through generic
supertypes and variance; a variable is fixed to the common supertype of
its lower bounds, or else its most specific upper bound. A generic call
that is an argument of another joins the outer system, as in K2, and so
do the branches of an `if` or `when` used as a value. A lambda argument
is checked after the other arguments, once the variables in its parameter
types are fixed, and its body constrains the result. Builder inference
(`buildList { add("x") }`) checks a lambda twice: first with variables
open, recording the subtyping on them, then once they are fixed
(`stubInference`).

**Smart casts.** Stable values (`val`s, `this`, final `val` properties
with fields, and local `var`s no closure assigns) are narrowed by `is`
and null checks, `as`, `!!`, `&&`/`||`, early returns, `when` branches
and guards, and the standard library's contracts. A `var`'s narrowing
follows the flow (`internal/check/flow.go`): assignments narrow, branches
join, and loops and `try`s drop what they assign. A value smart-cast to
an unrelated type has both types' members (`castView`). The checker marks
where a cast changes a value's class, and codegen casts there.

**Lambdas and references.** An expected function type gives a lambda its
parameter types. Lambdas for Java SAM interfaces and fun interfaces are
converted. Lambdas and local functions are checked in capture frames on a
stack, which record what they capture; a local function call passes the
callee's captures along, propagated to a fixed point. A callable
reference is parsed as a lambda with an empty body, which the checker
fills in once it knows the parameter types (`::f` is `{ p0 -> f(p0) }`),
so overloads, inference, defaults, SAM conversion and inlining treat it
as a lambda. Codegen then gives it its own `FunctionReferenceImpl` or
`PropertyReference1Impl` class, as `kotlinc` does.

**Operators** on numbers, `Char`, `Boolean` and `String` are JVM
instructions; otherwise the checker builds Kotlin's call (`a.plus(b)`) as
a `Call` marked `Operator`. Compound assignments and `++` are rewritten
into statements that evaluate receivers and indexes once.

**Multiplatform and contracts.** Code compiles for the JVM only: actual
declarations are ordinary ones, and expect declarations are only checked
against them (`expect.go`). `contract { }` calls are dropped
(`contracts.go`).

## Class files

Class files are version 52 (Java 8). The assembler
(`internal/classfile/code.go`) computes `StackMapTable` frames in the
same pass that emits code:

- It tracks each stack value's verification type and which locals hold
  values. Each local has a declared type, and slots are never reused, so
  merging paths never needs the class hierarchy.
- A label takes the state of the first edge reaching it and merges later
  ones; where branches leave different reference types (`if`, `when`,
  `?:` as values), codegen gives it the expression's static type.
- Backward jumps are checked against the recorded frame, so a mismatch is
  a compile error rather than a `VerifyError`.
- A handler's frame has the locals assigned when its `try` began, which is
  sound because slots aren't reused. A `try` used as a value first spills
  the stack to locals.

### The JVM ABI

Codegen writes what `kotlinc` writes, so code compiled by `kotlinc`
against `kotlinc`'s build of a library links against `kc`'s, and Java
sees the same API. `bench/apidump`, `bench/metadump` and `bench/annodiff`
compare a build's API, metadata and annotations with `kotlinc`'s; on
kotlinpoet they match, and kotlinpoet's tests pass on `kc`'s build (see
[Survey of real code](survey.md#kotlinpoet-end-to-end)). The less obvious
parts:

- **Classes.** Data classes, enums, objects, companions, class
  delegation (`$$delegate_N`), bridges and `name$default` methods as
  `kotlinc` writes them. A `finally` block is copied to every exit. A
  class implementing Kotlin's read-only collections gets the bridges and
  `UnsupportedOperationException` stubs `kotlinc` adds for Java
  (`codegen/collections.go`).
- **Anonymous, local and inner classes.** An object expression's class is
  declared and checked where the checker meets it, in a capture frame like
  a lambda's, so its captures (`this$0` included) are known; its
  constructor stores them in fields, and each method copies them to the
  locals the captured variables use. A local class is a named object
  expression. An inner class takes its outer instance first.
- **Properties.** A backing field exists if an accessor is default, uses
  `field` or is `external`. Delegated properties are rewritten by the
  parser into `x$delegate` and accessors calling `getValue`/`setValue`
  with a `KProperty` from `$$delegatedProperties`, left out where an
  inline `getValue` doesn't read it (`by lazy`).
- **Lambdas** that aren't inlined are private static methods created
  through `invokedynamic` and `LambdaMetafactory`, named as `kotlinc`
  names them (`main$lambda$0`); local functions are `main$g`. Without the
  standard library, function types are `kc/jvm/functions/FunctionN`
  interfaces written to the output directory.
- **Value classes** (`internal/types/value.go`, `codegen/value.go`) are
  their underlying value where the type is known, and boxed as `Any`,
  interfaces, type arguments and nullable types where `null` must stay
  apart; `types.Unboxed` decides. Members are static `-impl` methods, and
  functions taking value classes get `kotlinc`'s mangled names
  (`take-ggICe2c`).
- **ABI details.** Internal names end with the module name
  (`emit$kotlinpoet`), `META-INF/<module>.kotlin_module` lists facades,
  private members are reached through `access$` methods, `@JvmName`,
  `@JvmStatic`, `@JvmOverloads`, `@JvmField`, `@JvmMultifileClass` and
  `external` work as with `kotlinc`, and consts are folded and inlined.
  Interface bodies are default methods; `-jvm-default=enable` and
  `disable` add or use `DefaultImpls` instead.
- **Metadata** (`codegen/meta.go`, `kmeta/encode.go`) is written for
  facades, classes and synthetic classes, so `kotlinc` and
  `kotlin-reflect` see `kc`'s classes as Kotlin.
- **Generic signatures, annotations and null checks** are written as
  `kotlinc` writes them (`codegen/signature.go`, `annotations.go`,
  `nullcheck.go`), as are `LineNumberTable` and `LocalVariableTable`.

## Inlining

**The program's inline functions** are copied from the AST
(`codegen/inlinefun.go`): arguments go to new locals that the body's
parameters become, and a lambda the body only invokes (decided from the
body's syntax) isn't created but generated at the invoke, as the
caller's code. Since the AST is shared, a copy saves and restores its
variables' locals, for copies nested through lambdas. Reified type
arguments (`syntax.Call.Reified`) are substituted in the copy. The
function's own method, which other modules copy, marks reified operations
with `Intrinsics.reifiedOperationMarker`, as `kotlinc` does.

**Library inline functions** are copied from bytecode. The checker reads
a body on the first call that resolves to it. `internal/inline` follows
its control flow: a function-typed parameter the body only loads and
invokes is inlinable; one that is stored or passed on gets a lambda
object. Bodies already contain their own inline calls, so one level is
enough. A test analyzes the 2,739 inline functions of the default
imports: all can be inlined, and 1,909 of their 1,954 function-typed
parameters are inlinable.

Copying a body (`codegen/inline.go`):

- **Locals.** Each slot gets a new local per kind of value, since bodies
  reuse slots. Reference locals are `Object`, cast after each load,
  keeping the assembler's one-type-per-local rule.
- **Frames.** Each of the body's frames becomes a label with that frame,
  the caller's values below the body's on the stack.
- **Handlers** are copied with mapped ranges, after the caller's stack is
  spilled.
- **Reified operations** that markers flag use the call's type argument.
  An object using the type parameters (`filterIsInstance`'s
  `{ it is R }`) is a template class (`inline.Template`) that each copy
  copies, renamed in its constant pool, with the markers made `nop`s so
  offsets and frames stay valid.

An inlined lambda's body is generated at each invoke, with no capture
boundary; `return@label` jumps to its end, and a plain `return`, `break`
or `continue` leaves the enclosing function's code.

## Coroutines

A suspend function takes a `Continuation` last and returns an `Object`,
its result or `COROUTINE_SUSPENDED`, as with `kotlinc`. One whose every
suspension point is a tail call passes its continuation on. Any other is
a state machine, built in the single pass of code generation, without
transforming bytecode afterwards:

- On entry it finds or creates its continuation (a `ContinuationImpl`
  with `label` and spill fields). If `label` isn't 0 it jumps to a
  `tableswitch` at the end of the method, since resume points aren't known
  before the body is generated.
- A suspension point spills the stack (`Code.Spill`), saves locals in
  fields (`L$0`, `I$0`...), sets `label` and calls. An assembler hook,
  `Code.OnCall`, adds the suspension check after the next call
  instruction, so all of codegen's call paths work unchanged.
- The resume point is a label with the dispatch's frame
  (`Code.NewFrameLabel`) that restores the locals; it is a gap in the
  surrounding `try` ranges.

A point saves what `kotlinc` 2.4 saves. Dead references go through
`SpillingKt.nullOutSpilledVariable`, and a liveness analysis at the end
turns the calls for live ones into `nop`s. Continuation classes carry
`@DebugMetadata`. A suspend lambda is its own continuation, a
`SuspendLambda` subclass. Suspension points in inlined code belong to the
caller's state machine; library inline suspend functions are copied from
their `name$$forInline` methods, whose `InlineMarker` calls bracket each
point, and the program's public inline suspend functions get such
methods for `kotlinc`.

`TestCoroutineShapes` compiles the coroutine test programs with both
compilers and compares their continuation and lambda classes; the
differences are listed in `testdata/coroutine-shapes.golden`.

## Compiler plugins

With `-compose`, `-serialization` and `-parcelize`, `kc` compiles as
`kotlinc` does with those plugins. Each declares the members its plugin
adds as source before the file is checked, and fills in bodies that
depend on property types once those are known:

- **Compose** (`codegen/compose.go`, `skip.go`, `stability.go`,
  `singletons.go`): `$composer`, `$changed` and `$default` parameters,
  restart groups that skip unchanged arguments, and composable lambdas,
  with those capturing nothing in `ComposableSingletons`.
- **kotlinx.serialization** (`check/serialization.go`, `codegen/serial.go`):
  the `$serializer` object, `serializer()`, `write$Self` and the
  deserialization constructor.
- **Parcelize** (`check/parcelize.go`): `writeToParcel`, a `C$Creator`
  class and `CREATOR`.
