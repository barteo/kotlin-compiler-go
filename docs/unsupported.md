# Unsupported Kotlin

`kc` compiles Kotlin 2 for the JVM with `kotlinc`'s ABI (see [The JVM
ABI](design.md#the-jvm-abi)). This lists what it doesn't compile, or
compiles only in part, and then, in [Differences from
kotlinc](#differences-from-kotlinc), where what it compiles differs from
`kotlinc`'s output. What isn't listed is meant to work as with `kotlinc`.
[Conformance](conformance.md) measures how far that holds on `kotlinc`'s
own tests.

## The language

- Secondary constructors of enum classes. They compile, but to a
  constructor the JVM verifier rejects.
- A `try` as an argument of an enum entry's constructor call.
- Visibility isn't checked, but for private constructors: private,
  protected and internal declarations can be used where Kotlin doesn't
  allow it. It still shapes the class files as with `kotlinc`.
- Reified type parameters of members of interfaces and value classes
  (those of final members and member extensions of classes work).
- Properties of a `@file:JvmMultifileClass` class that are neither
  `private` nor `const`.
- `@JvmStatic` on extensions of objects and on delegated properties,
  which is reported as unsupported.
- Local `external` functions, which are reported as unsupported.
- Omitted arguments of a library's annotation class, and annotation
  parameters of unsigned integer types.
- Builder inference that can't check a lambda's body with the type
  variables open, as a member of a value of one of them whose type
  nothing gives yet: it needs a type argument or an expected type.
- Context parameters, `context(a: A) fun f()`, work on top-level and
  extension functions, in context function types and their lambdas,
  and with the standard library's `context(a) { }`. These are reported
  as unsupported: context parameters of members, properties, local
  functions and classes; the context receivers that came before them,
  `context(A)`; calls of a library's functions with context
  parameters; and callable references to functions with them. Where
  two implicit values at the same level fit a context parameter,
  `kotlinc` reports an ambiguity, and `kc` passes one of them; nor does
  `kc` prefer a function without context parameters to an otherwise
  equally specific one with them, as `kotlinc` does.

## Without the standard library

Without `kotlin-stdlib` on the class path, only these of it are
available, built in: `println`, `print`, `mutableListOf()`, `listOf()`
and the other factories without arguments, `map[key] = value`, the
array factories (`emptyArray`, `arrayOfNulls`, `toTypedArray()`),
`Char(code)`, `String(chars)`, `String.trim()`, the `Char` functions
that are `java.lang.Character` methods, and `require`, `check`, `error`
and `TODO`. Use the Java classes directly for the rest. Also:

- Ranges work only in `for` loops and `in` checks; they aren't values
  (`val r = 1..10` needs `IntRange`).
- `X::class` isn't a `KClass`; `X::class.java` works.
- Suspend functions and coroutines aren't available.
- Programs that use function types need the interfaces `kc` writes to
  `kc/jvm/functions/` in the output directory.

## Inlining

- A library's inline function whose body has an `invokedynamic` call
  other than `StringConcatFactory`'s, such as an indy lambda's
  (`LambdaMetafactory`) that refers to a private method of the library's
  class, can't be inlined: the call is reported.
- Inline functions that `kotlinc` compiled for JVM 9 and later, with
  `StringConcatFactory` string templates, are copied into `kc`'s version
  52 classes, which then need Java 9 to run. `kotlinc` refuses to inline
  them into a lower JVM target.

## Coroutines

- Two references of a suspend function type to the same suspend
  function aren't equal if they ignore its result, or are to an inline
  suspend function or to one that returns a value class.

## kotlinx.serialization (`-serialization`)

Not supported: generic classes; interfaces annotated `@Serializable`
that aren't sealed; local classes; superclasses from other modules;
more than 32 properties; custom serializers of properties
(`@Serializable(with = ...)` on a property, `@Contextual`,
`@UseSerializers`); `@EncodeDefault`; the annotations that formats read
(`@JsonNames`, `@ProtoNumber`...); arrays and the other types with
built-in serializers (`Duration`, `Uuid`...). Properties can be of the
primitive types, `String`, enums, `@Serializable` classes and objects,
interfaces (polymorphically), and `List`, `Set`, `Map`, `HashSet`,
`HashMap`, `Pair` and `Triple` of these, each nullable too.

Without `-serialization`, only classes annotated `@Serializable(with =
S::class)` get their `serializer()`.

## Parcelize (`-parcelize`)

Not supported: other types than `String`, the primitive types, enums,
`Parcelable`s, `Serializable`s, `Array<String>`, and `List`,
`MutableList`, `ArrayList`, `Set`, `MutableSet`, `LinkedHashSet`, `Map`,
`MutableMap`, `LinkedHashMap` and `HashMap` of these (so not other
arrays, `CharSequence`, `Bundle`...); custom `Parceler`s;
`@TypeParceler`, `@WriteWith` and `@RawValue`; generic `@Parcelize`
classes.

## Compose (`-compose`)

Not supported yet:

- non-local jumps from the lambdas of inline composables;
- `key` with a spread array of keys;
- composable calls in `catch` and `finally` blocks and in local
  functions (calls in a `try` are an error, as with `kotlinc`);
- references to composables;
- composable lambdas with more than 9 parameters (which `kotlinc`'s
  plugin fails to compile);
- suspend composable function types, classes that implement composable
  function types, and `is` and `as` with them;
- lambdas converted to fun interfaces whose composable method returns a
  value or that use `this`;
- composables with bodies in interfaces, and composable extension
  properties;
- `@ExplicitGroupsComposable`.

Without `-compose`, calls of libraries' composables aren't supported.

## Differences from kotlinc

Code that compiles, but not to what `kotlinc` writes, by design or not
yet done. [Design](design.md) describes how `kc` compiles it.

**Checking**

- The common supertype of unrelated types is one class or interface (for
  `Int` and `String`, `Any`), not an intersection type.
- A lambda's parameter types are fixed from the constraints known when
  the lambda is reached; K2 picks the order from the whole system, so it
  infers some combinations of lambdas `kc` can't.
- Java module exports aren't checked, and the Java members Kotlin
  replaces (`Number.intValue()`) aren't hidden.
- A `return@label` without a value makes a lambda return `Unit`, even if
  its last expression has a value.
- Without an expected type, a reference to a generic or vararg function
  is an error.

**Classes and members**

- A captured `var` is kept in a one-element array, in lambdas, objects
  and coroutines, not a `kotlin.jvm.internal.Ref`.
- Lambda methods take and return erased types (`Object`), where
  `kotlinc`'s take the lambda's own types and let `LambdaMetafactory`
  box. They take the enclosing object before the captures, and lambdas in
  objects take the object rather than read `INSTANCE`. A local function
  in an `init` block captures the constructor parameters it uses.
- Function values (not lambdas) converted to Java SAM interfaces use
  `$sam$` wrapper classes, where `kotlinc` uses `invokedynamic`.
- A reference to a library's inline function (`String::uppercase`) is a
  lambda, with a lambda's `toString()` and `equals()`, and so is a
  suspend reference whose `invoke` `kotlinc` makes a state machine (one
  that ignores the result, or to an inline suspend function or one
  returning a value class).
- A `when` over an enum compares entries with `equals`, not through a
  `$WhenMappings` ordinal switch.
- Interfaces default to `-jvm-default=no-compatibility`; `kotlinc`'s
  default is `enable`.
- A companion's properties are fields of the companion, not private
  statics of the outer class (consts and `@JvmField` ones aside). An
  interface companion's lambdas and local functions are static methods of
  the interface.
- No `PermittedSubclasses` attributes, and `InnerClasses` entries only
  for the file's own nested classes, not for those a class refers to.
- A value class's inherited members are called on the box, and `==`
  boxes both values and calls `equals` rather than `equals-impl0`.
- A property delegated to a final property or a property reference keeps
  the delegate in a field; `kotlinc` reads it each time through
  `getX$delegate`. A local delegated property's accessors are inlined
  where used. The program's own inline `getValue` always gets a
  `KProperty`.
- The spread operator copies with `clone()` and `System.arraycopy`, not
  `Arrays.copyOf` and `SpreadBuilder`. `Array(n) { }` calls its init
  function as an object; `kotlinc` inlines it.

**Inlining**

- Inlined lambdas take and return boxed `Object`s; arguments always go
  through locals, with a cast after each reference load; and a
  `$default` body's mask is tested at run time rather than folded.
- Lambdas passed for crossinline parameters are objects of the library's
  template classes (`ComparisonsKt__ComparisonsKt$thenBy$1`); `kotlinc`
  regenerates those classes with the lambda inlined. `kc` copies
  template classes only for reified type parameters and in public inline
  functions' methods. The same goes for a crossinline suspend lambda an
  object captures (`Flow.map`): it is a `SuspendLambda` passed to the
  copy.
- An object expression in the program's inline function is one class for
  all copies, using a crossinline lambda as a function object; `kotlinc`
  copies the class per call.
- In the method of a function with a reified type parameter, a library
  object that uses it (`filterIsInstance`'s `{ it is R }`) is copied
  erased, so code in another module that copies the method filters
  nothing; `typeOf<List<T>>()` there throws.
- An open inline member and an inline member of an interface are
  ordinary methods.

**Coroutines**

- The dispatch is at the end of the method, and resume points restore
  locals in reverse order.
- Copied inline functions' locals are unnamed, so `@DebugMetadata`
  doesn't list them, and they are saved only while live. The lines after
  suspension points come from `kc`'s line tables.

**Debug information and metadata**

- Inlined code keeps the line of its call and there's no SMAP
  (`SourceDebugExtension`), so a debugger can't step into `let` or
  `forEach`. Inlined functions' variables aren't named (`x$iv`, `$i$f$`
  and `$i$a$` markers).
- The metadata version is 1.9, which `kotlinc` 1.9 and later read, and
  declarations' annotations aren't repeated in metadata, as `kotlinc` 2.4
  does.
- A value class's synthesized members' signatures are always the static
  `-impl` methods; for an internal value class, and for one in common
  (multiplatform) sources, `kotlinc` 2.4.20 writes the instance methods.
- Synthetic classes are named otherwise: `kotlinc` numbers every lambda
  of a function, `kc` only those that become classes, and `kc` copies
  an inline function's objects and continuations as its own classes
  (`wrapCause$default$1`, `lookAhead$1`) where `kotlinc` has `$$inlined$`
  copies.
- A local class's types that use the enclosing function's type
  parameters name them; `kotlinc` writes their ids, which the class
  doesn't declare.
- An alias of an alias is written expanded: `kc` keeps no
  abbreviations.
- A private property initialized with an object expression has the
  object's supertype, not its anonymous type.
- A class-path generic value class whose underlying type is its type
  parameter isn't known as one until it is loaded.
- Annotations on local variables, type parameters, types and
  expressions aren't written, nor are `@[A B]` lists.
