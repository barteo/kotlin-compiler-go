# Unsupported Kotlin

`kc` compiles Kotlin 2 for the JVM with `kotlinc`'s ABI (see [The JVM
ABI](design.md#the-jvm-abi)). This lists what it doesn't compile, or
compiles only in part, and then, in [Differences from
kotlinc](#differences-from-kotlinc), where what it compiles differs from
`kotlinc`'s output. What isn't listed is meant to work as with `kotlinc`.
[Conformance](conformance.md) measures how far that holds on `kotlinc`'s
own tests.

## The language

- Reified type parameters of members of interfaces and value classes
  (those of final members and member extensions of classes work).
- `lateinit` properties of a `@file:JvmMultifileClass` class that
  aren't `private`.
- Local `external` functions, which are reported as unsupported.
- Opt-in requirements (`@RequiresOptIn`, `@SubclassOptInRequired`) and
  `@Deprecated` with level `ERROR` or `HIDDEN` are checked only for the
  program's own declarations: the markers and deprecations of the class
  path, the standard library's included, aren't read from class files.
  The class path's functions and members deprecated `HIDDEN` stay hidden,
  as `kotlinc` compiles them synthetic, but its classes don't.
- Builder inference of a builder nested in another whose lambda combines
  values of both builders' variables, as `select(this@outer.get(),
  get())`, and where Kotlin forks the constraint system for a call in the
  lambda: it needs a type argument or an expected type.

## Without the standard library

Without `kotlin-stdlib` on the class path, only these of it are
available, built in: `println`, `print`, the array factories
(`arrayOf`, `emptyArray`, `arrayOfNulls`), `Char(code)`,
`String(chars)`, the `Char` functions that are `java.lang.Character`
methods, and `require`, `check`, `error` and `TODO` (which throws an
`UnsupportedOperationException`). Collection factories and types
(`listOf`, `mutableListOf`, `MutableList`), `map[key] = value`,
`toTypedArray()` and `String.trim()` need the standard library. Use the
Java classes directly for the rest. Also:

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
  class, can't be inlined. A call of one with reified type parameters,
  or of an inline-only one, is reported; another is called as a
  function instead, and its lambdas can't return from the caller.
- Inline functions that `kotlinc` compiled for JVM 9 and later, with
  `StringConcatFactory` string templates, are copied into `kc`'s version
  52 classes, which then need Java 9 to run. `kotlinc` refuses to inline
  them into a lower JVM target.

## Coroutines

- Two references of a suspend function type to the same suspend
  function aren't equal if they ignore its result, or are to an inline
  suspend function or to one that returns a value class of a reference
  or nullable type (not `Int` or `Long`).

## kotlinx.serialization (`-serialization`)

Not supported: generic classes; interfaces annotated `@Serializable`
that aren't sealed; local and inner classes and companion objects;
classes without a primary constructor or that delegate an interface
(`by`); superclasses from other modules; more than 32 properties;
properties in the class body that are `lateinit`, set in an `init` block
rather than initialized, or have accessors of their own; some ways of
naming serializers of properties (a generic class in `@Serializable(with
= ...)` on a property, `@Contextual`, `@UseSerializers`,
`@UseContextualSerialization`); `@Polymorphic`, `@KeepGeneratedSerializer`
and `@MetaSerializable`; the annotations that formats read
(`@JsonNames`, `@ProtoNumber`...); arrays and the other types with
built-in serializers (`Duration`, `Uuid`...). Properties can be of the
primitive types, `String`, enums, `@Serializable` classes, objects and
value classes, `UInt`, `ULong`, `UShort`, `UByte`, interfaces
(polymorphically), and `List`, `Set`, `Map`, `HashSet`,
`HashMap`, `Pair` and `Triple` of these, each nullable too.

Without `-serialization`, only classes annotated `@Serializable(with =
S::class)` get their `serializer()`.

## Parcelize (`-parcelize`)

A class that kotlinc reports an error for, or fails to compile, is left
to kotlinc: one with a property of a type that the plugin can only write
with `writeValue` and that isn't annotated `@RawValue`, such as a value
class that isn't `Parcelable` or `Serializable`, or of a type parameter
whose bound is a collection; a `@TypeParceler` or `@WriteWith` whose
`Parceler` isn't an object or doesn't fit the type, or two
`@TypeParceler`s for one type; a `Parceler` companion object of another
type than the class's; a `@DataClass` type whose data class's
constructor or properties the class can't see, or whose data class, or
one in it, is another module's, such as `Pair`, for which kotlinc's
plugin finds no properties and crashes.

## Compose (`-compose`)

Not supported yet:

- suspend, `tailrec`, `@JvmStatic` and `@JvmOverloads` composable
  functions, and those of value classes;
- composable properties without a getter with a body, with an inline
  getter, delegated, or of interfaces and value classes;
- composable functions of an interface a class delegates (`by`);
- non-local jumps from the lambdas of inline composables;
- `key` other than with keys and a lambda, such as with a spread array
  of keys;
- composable calls in `catch` and `finally` blocks and in local
  functions (calls in a `try` are an error, as with `kotlinc`);
- references to composables;
- composable lambdas with more than 9 parameters (which `kotlinc`'s
  plugin fails to compile);
- suspend composable function types, classes that implement composable
  function types, and `is` and `as` with them;
- lambdas converted to fun interfaces whose composable method returns a
  value or that use `this`;
- composables with bodies in interfaces;
- `@ExplicitGroupsComposable`.

Without `-compose`, calls of libraries' composables aren't supported.

## Metro (`-metro`)

`kc` implements Metro 1.5.2's dependency graphs: `@DependencyGraph` and its factory, with `@Provides` and `@Includes` parameters; `@Inject` constructors and objects, `@Provides`, `@Binds`, qualifiers and `@SingleIn` scopes; requests as `() -> T`, `Provider<T>` and `Lazy<T>`; set and map multibindings, including `@ElementsIntoSet`, `@Multibinds` and wrapped map values; `@BindingContainer`; assisted injection; member injection; `@ContributesTo`, `@ContributesBinding`, `@ContributesIntoSet` and `@ContributesIntoMap` with `replaces` and `excludes`, from this module or the class path; and `@GraphExtension`s with their factories.

Not supported, and reported as such:

- custom annotations (the `custom-*` options), and options that change Metro's output, other than `enabled`, `contributes-as-inject` and `enable-function-providers`;
- `@Provides` extension functions, and `@Provides` members of an
  interface or an abstract class outside its companion object;
- graphs with type parameters, and graph factories that are abstract classes;
- injecting a generic class where the requested type's arguments are type
  parameters, and contributed interfaces with type parameters;
- javax, Jakarta, Dagger and Guice interop;
- member injection into classes from the class path;
- member injection that depends on the class itself, as `@Inject lateinit var s: Lazy<S>` in `S`;
- `createDynamicGraph` and `createDynamicGraphFactory`, and calls of `createGraph` and `createGraphFactory` whose type argument is inferred;
- graph members that are extensions, or functions with parameters that
  neither inject members nor create a graph extension;
- graph functions that create a graph extension from parameters, other than its factory's.

`kc` writes no `$MetroFactory` classes, so a graph that `kotlinc` compiles can't use a module that `kc` compiles. A graph that `kc` compiles can use modules that either compiler compiled.

## Differences from kotlinc

Code that compiles, but not to what `kotlinc` writes, by design or not
yet done. [Design](design.md) describes how `kc` compiles it.

**Checking**

- Intersection types (`A & B`) are inferred only for the shared
  supertypes of Kotlin classes and generic Java ones (a Java class's
  non-generic interfaces only when nothing else is shared), smart casts between a class type and an unrelated
  interface, or of such an intersection to another (`C & A & B`),
  smart casts of a value of a type parameter's type to a class
  type or `String` (`String & T`), identity or safe equality of two
  generic class types (`Box<T> & Box<A>`), and incomparable upper bounds of a
  variable. Other smart casts are to the target type alone: a class type
  of a generic or Java type, a type parameter to another. A call on an
  intersection takes the first component with a member of the name
  whose parameters take its arguments, and a property read on one is
  the intersection of its types only over components of the same class;
  inference sees an intersection as its first component, the declared
  type, where K2 forks over the components. A variable whose upper constraints don't meet its declared
  bounds, as `T` of `fun <T : Foo> foo(): T?` for a `Bar?`, is inferred
  from the constraints alone, where Kotlin infers `Foo & Bar`. A smart cast of a `T` to a primitive type or a value class is
  that type alone, which a generic call expecting a `T` takes only
  directly, as `listOf(o)`, not through another call, as `mapOf(o to o)`
  for an `Int` `o`. The common
  supertype of two builtin types is `Any`, or `Number` for numbers (for
  `Int` and `String`, Kotlin's is `Comparable<*> & Serializable`).
- A lambda's parameter types are fixed from the constraints known when
  the lambda is reached; K2 picks the order from the whole system, so it
  infers some combinations of lambdas `kc` can't.
- `@OnlyInputTypes` (`contains`, `get`, `assertEquals`) is checked only
  when the types a call's variable is constrained by have no captured
  types (star projections), and the inferred type isn't an
  intersection's component.
- Of the Java collection members K2 gives Kotlin's signatures, `kc` maps
  `contains`, `remove`, `containsAll`, `removeAll`, `retainAll`,
  `indexOf`, `lastIndexOf`, and a map's `containsKey`, `containsValue`,
  `get` and `remove`; others, such as `getOrDefault`, keep Java's.
- Of K2's predefined nullability of JDK members, only `java.util.Optional`'s
  (`of`, `ofNullable`, `empty`, `get`, `ifPresent`) is applied. A nullable
  argument for a Java method's `T!` makes `T` nullable, except in an
  in-projected type argument, where kc's substituted explicit type
  arguments stand for K2's equality constraints.
- Java module exports aren't checked, and the Java members Kotlin
  replaces (`Number.intValue()`) aren't hidden.
- Without an expected type, a reference to a vararg function, or to a
  generic function whose type parameters its receiver's type doesn't
  give (`Box<Int>::first` for `fun <T> Box<T>.first()` is fine), is an
  error.
- Where annotations are written, and whether they repeat, isn't checked
  for Java annotations without `@Target` or from Java sources, nor for
  use-site targets on local declarations and expressions (`@field:` on a
  local variable), nor across a typealias's expansion (`@A T` where
  `typealias T = @A Int`). The arguments of a
  library's `SOURCE` annotation aren't checked, so they need not be
  constants, nor are annotations on setter parameters, `set(@A v)`.

**Classes and members**

- A captured `var` is kept in a one-element array, in lambdas, objects
  and coroutines, not a `kotlin.jvm.internal.Ref`.
- Lambda methods take and return erased types (`Object`), where
  `kotlinc`'s take the lambda's own types and let `LambdaMetafactory`
  box. They take the enclosing object before the captures, and lambdas in
  objects take the object rather than read `INSTANCE`. A local function
  in an `init` block, and a lambda in a property's initializer, take the
  values of the `val` constructor parameters they use, where `kotlinc`'s
  read their fields.
- Function values (not lambdas) converted to Java SAM interfaces use
  `$sam$` wrapper classes, where `kotlinc` uses `invokedynamic`.
- A reference to a library's inline function (`String::uppercase`), or
  a property reference used as a function value (`val f: (P) -> Int =
  P::x`), is a lambda, with a lambda's `toString()` and `equals()`, and
  so is a suspend reference whose `invoke` `kotlinc` makes a state machine (one
  that ignores the result, or to an inline suspend function or one
  returning a value class).
- A `when` over an enum compares entries with `equals`, not through a
  `$WhenMappings` ordinal switch.
- Interfaces default to `-jvm-default=no-compatibility`; `kotlinc`'s
  default is `enable`.
- A companion's property fields are package-private statics of the
  outer class, which the companion's accessors read directly, rather than
  private ones read through `access$getX$cp`. Where the outer class has a
  property of the same name, `kotlinc` renames the outer class's field
  (`x$1`); `kc` keeps the companion's field in the companion instead. An
  interface companion's fields are instance fields, not static ones, and
  the lambdas and local functions of its initializers and `init` blocks
  are static methods of the interface.
- No `PermittedSubclasses` attributes, which `kotlinc` writes for
  `-jvm-target 17` and later, and `InnerClasses` entries only
  for the file's own nested classes, not for those a class refers to.
- A value class's inherited members are called on the box, and `==`
  boxes both values and calls `equals` rather than `equals-impl0`.
- A property delegated to a final property or a property reference keeps
  the delegate in a field; `kotlinc` reads it each time through
  `getX$delegate`. A local delegated property's accessors are inlined
  where used. The program's own inline `getValue` always gets a
  `KProperty`.
- The spread operator copies with `clone()` and `System.arraycopy`, not
  `Arrays.copyOf` and `SpreadBuilder`. `Array(n, ::f)` calls its init
  function as an object; `kotlinc` inlines it.

**Inlining**

- A library's inline functions, inlined from their bytecode, have their
  lambdas take and return boxed `Object`s, their arguments always go
  through locals, with a cast after each reference load, and their
  `$default` bodies' masks tested at run time rather than folded.
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
  nothing.
- A library interface's inline member is called as an ordinary method.
  In the program, open and interface inline members are errors, even
  with `@Suppress("DECLARATION_CANT_BE_INLINED")`, which `kotlinc`
  accepts.

**Coroutines**

- The dispatch is at the end of the method.
- Copied inline functions' locals are unnamed, so `@DebugMetadata`
  doesn't list them, and those dead at a suspension point are saved, not
  nulled out as `kotlinc` does. The lines after
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
  `-impl` methods; for one in common (multiplatform) sources, `kotlinc`
  writes the instance methods.
- Synthetic classes are named otherwise: `kotlinc` numbers every lambda
  of a function, `kc` only those that become classes, and `kc` copies
  an inline function's objects and continuations as its own classes
  (`wrapCause$default$1`, `lookAhead$1`) where `kotlinc` has `$$inlined$`
  copies.
- A local class's types that use the enclosing function's type
  parameters name them; `kotlinc` writes their ids, which the class
  doesn't declare.
- Types written with a typealias, and an alias of an alias, are written
  expanded: `kc` keeps no abbreviations.
- A class's private property initialized with an object expression has
  the type `Outer$1` in metadata, a class that doesn't exist, rather than
  its anonymous class (`Outer$prop$1`).
- A generic value class whose underlying type is a type parameter
  bounded by a primitive type (`value class W<T : Int>(val s: T)`) is
  unboxed as the boxed primitive (`Integer`); `kotlinc` passes the
  primitive (`int`) to its `-impl` methods.
- Annotations on type parameters and types aren't written, nor are
  `@[A B]` lists.

**Compiler plugins**

- The parcelize plugin reads a `FileDescriptor` with
  `Parcel.readFileDescriptor()`, which returns a `ParcelFileDescriptor`.
  `kotlinc` casts it to `FileDescriptor`, which fails, except for the
  elements of collections and arrays, which it leaves uncast; `kc` casts
  those too.
- A custom `Parceler`'s `write`, a member extension, is called through an
  inlined `with`, which leaves a `Unit` loaded and popped after the call
  that `kotlinc` doesn't have, and a `Parceler` companion's `newArray` is
  cast to the array type, which `kotlinc` doesn't. kotlinc reports
  `@TypeParceler` and `@WriteWith` in a class that isn't `@Parcelize`
  as errors; `kc` ignores them.
- `kotlinc`'s plugin code casts values to the types they are passed as
  where those are supertypes, as a list read to `List` or the receiver of
  `toPersistentList` to `Iterable`; `kc`'s doesn't, and casts the
  elements of a value class that it reads into a list to the class.
- `Uuid` is written with an inlined `toLongs`, whose lambda's arguments
  `kc` boxes (see Inlining).
