# Unsupported Kotlin

`kc` compiles Kotlin 2 for the JVM with `kotlinc`'s ABI (see [The JVM
ABI](design.md#the-jvm-abi)). This lists what it doesn't compile, or
compiles only in part, and then, in [Differences from
kotlinc](#differences-from-kotlinc), where what it compiles differs from
`kotlinc`'s output. What isn't listed is meant to work as with `kotlinc`.
[Conformance](conformance.md) measures how far that holds on `kotlinc`'s
own tests.

## The language

- Warnings: `kc` reports only `DEPRECATION`, `OVERRIDE_DEPRECATION`,
  `TYPEALIAS_EXPANSION_DEPRECATION`, `DEPRECATED_JAVA_ANNOTATION`,
  `INLINE_CLASS_DEPRECATED`, `OPT_IN_USAGE`, `OPT_IN_OVERRIDE`,
  `ANNOTATION_TARGETS_ONLY_IN_JAVA`, `DSL_MARKER_APPLIED_TO_WRONG_TARGET`
  (not on a lambda), `POTENTIALLY_NON_REPORTED_ANNOTATION`,
  `ERROR_SUPPRESSION`,
  `UNNECESSARY_SAFE_CALL`, `UNNECESSARY_NOT_NULL_ASSERTION`,
  `NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION`,
  `NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE`, `USELESS_CAST`,
  `UNCHECKED_CAST`, `SENSELESS_COMPARISON`, `SENSELESS_NULL_IN_WHEN`,
  `USELESS_ELVIS`, `USELESS_ELVIS_LEFT_IS_NULL`,
  `USELESS_ELVIS_RIGHT_IS_NULL`, `OVERRIDE_BY_INLINE`,
  `NOTHING_TO_INLINE`, `FINAL_UPPER_BOUND`, `REDUNDANT_PROJECTION`,
  `MISPLACED_TYPE_PARAMETER_CONSTRAINTS`,
  `PLATFORM_CLASS_MAPPED_TO_KOTLIN`, `NO_TAIL_CALLS_FOUND`,
  `NON_TAIL_RECURSIVE_CALL`, `TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED`,
  `USELESS_IS_CHECK`, `IMPOSSIBLE_IS_CHECK_WARNING`,
  `IMPOSSIBLE_IS_CHECK_RELYING_ON_NULL_WARNING`, `CAST_NEVER_SUCCEEDS`,
  `NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL`,
  `SAFE_CAST_RELYING_ON_NULL`, `UNSAFE_CAST_RELYING_ON_NULL`,
  `DEPRECATED_IDENTITY_EQUALS`, `IMPLICIT_BOXING_IN_IDENTITY_EQUALS`,
  `FORBIDDEN_IDENTITY_EQUALS_WARNING` (not of platform types),
  `ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS`,
  `REDUNDANT_CALL_OF_CONVERSION_METHOD`, `EMPTY_RANGE`,
  `UNUSED_EXPRESSION`, `UNUSED_LAMBDA_EXPRESSION`,
  `EXTENSION_SHADOWED_BY_MEMBER`,
  `EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE`,
  `PARAMETER_NAME_CHANGED_ON_OVERRIDE`,
  `DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES`,
  `DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE`,
  `PROPERTY_HIDES_JAVA_FIELD`, `ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY`,
  `ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY`,
  `NO_REFLECTION_IN_CLASS_PATH`, `DIVISION_BY_ZERO`,
  `REDUNDANT_ELSE_IN_WHEN`, `INCOMPATIBLE_ENUM_COMPARISON`,
  `NON_FINAL_MEMBER_IN_FINAL_CLASS`, `NON_FINAL_MEMBER_IN_OBJECT`,
  `REDUNDANT_OPEN_IN_INTERFACE`, `REDUNDANT_MODIFIER`,
  `USELESS_VARARG_ON_PARAMETER`, `DUPLICATE_BRANCH_CONDITION_IN_WHEN`,
  `REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_FUNCTION`,
  `REDUNDANT_SPREAD_OPERATOR_IN_NAMED_FORM_IN_ANNOTATION`,
  `INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL`, `REDUNDANT_NULLABLE`,
  `REDUNDANT_EXPLICIT_BACKING_FIELD` (not of a field the property can't
  have),
  `RETURN_IN_FUNCTION_WITH_EXPRESSION_BODY_WARNING`,
  `LEAKED_IN_PLACE_LAMBDA` and `WRONG_INVOCATION_KIND` (not of a
  parameter used in a callable reference, a safe call or a context
  argument, nor the invocation kind where its calls are in a loop that
  jumps, a lambda that jumps, or a `when` without `else` that isn't
  exhaustive, or in a function that never returns normally),
  `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING` (not
  of a class that doesn't override the function),
  `UNNECESSARY_LATEINIT` (not where an object expression or a local
  class reads the property, nor of a type parameter's type, nor for
  constructors whose `this(...)` calls loop),
  `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` and
  `WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING` (not
  through a typealias, nor of an inner class of a generic nested
  class, nor of a local class inside a generic class),
  `ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING` (not of a
  constructor call),
  `CONTEXTUAL_OVERLOAD_SHADOWED` (not where the context parameters'
  types mention type parameters), `EQUALITY_NOT_APPLICABLE_WARNING`
  (of smart-cast values, not of enums nor an identity comparison of a
  value without identity), `CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING` (of
  an inherited var's setter), `SYNCHRONIZED_ON_INLINE`,
  `INFERRED_INVISIBLE_RETURN_TYPE_WARNING`,
  `INFERRED_INVISIBLE_WHEN_TYPE_WARNING`,
  `INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING` and
  `INFERRED_INVISIBLE_VARARG_TYPE_ARGUMENT_WARNING` (only of a private
  top-level class of another file or a package-private Java class of
  another package, or a class nested in one, not of a private or
  protected nested class, nor of an `if` or `when` that an expected
  type gives a type),
  `JAVA_CLASS_ON_COMPANION` (not through an alias of a built-in type),
  `DANGEROUS_CHARACTERS` (not of a function type's parameter names, nor
  of a destructuring `for` loop's) and, in
  explicit API warning mode, `NO_EXPLICIT_VISIBILITY_IN_API_MODE_WARNING`
  and `NO_EXPLICIT_RETURN_TYPE_IN_API_MODE_WARNING`; and those only
  where the types make them certain, so that `-Werror` doesn't reject
  what `kotlinc` compiles: the null checks never of a platform type, a type parameter
  or a type it couldn't resolve, and `UNCHECKED_CAST` only of a cast to
  a type parameter that isn't reified, to a generic type from `Any`
  or a star-projected supertype, to a function type from another of
  its class, or to a generic type whose type arguments its operand's
  type doesn't tell, judged as `kotlinc` does but only where that
  type's arguments map to the target class's type parameters one by one,
  without projections, and the operand isn't smart cast. Parameter names are compared only where they are stable,
  of the program's and Kotlin libraries' functions, not Java methods,
  and `DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES` and
  `DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE` only of the program's
  Kotlin members. `PLATFORM_CLASS_MAPPED_TO_KOTLIN` is
  reported where the Java class is named, not through a typealias or an
  inferred type, and `NO_TAIL_CALLS_FOUND` not where a recursive call in
  tail position may be a tail call to `kotlinc`, as one on another
  receiver. Type checks and casts are judged as `kotlinc` judges them,
  by the declared type and then by its smart casts, but never where a
  platform type or a type `kc` couldn't resolve is involved. Unused
  expressions are judged as `kotlinc` does, but never property reads,
  calls or operators. Shadowed extensions are judged by the members of
  the program's classes and of the class path's Kotlin classes, not of
  Java classes or built-in types such as `List` and `Int`.
  `DIVISION_BY_ZERO` isn't reported in annotation arguments, and
  `REDUNDANT_ELSE_IN_WHEN` only of a `when` over a Boolean, an enum or
  a sealed class, not of one exhaustive through an `is` check of the
  subject's own type, or over a type parameter's value.
  `INCOMPATIBLE_ENUM_COMPARISON` is reported as `kotlinc` reports it,
  of an enum and a class or interface it isn't related to, by the
  operands' types after smart casts and a type parameter's bounds, but
  not where a platform type is involved. `ERROR_SUPPRESSION` is reported, as `kotlinc` 2.4.21
  does, for every error name in a `@Suppress`, but not in one on a type
  or a supertype. With `-Wextra`, as `kotlinc`, it also reports
  `UNREACHABLE_CODE`, after a statement that always jumps (not the dead
  code of a class's initialization order), `UNUSED_VARIABLE`,
  `VARIABLE_NEVER_READ`, `ASSIGNED_VALUE_IS_NEVER_READ` and
  `VARIABLE_INITIALIZER_IS_REDUNDANT`, the last two only where no
  statement that may read the value comes before it is overwritten or
  out of scope, through `if` and `when` but not loops or `try`, and,
  for a variable a lambda, a local function or a local class uses,
  only before the first of them and outside loops,
  `UNUSED_ANONYMOUS_PARAMETER`, but not of a parameter whose name the
  lambda's body uses for another variable,
  `REDUNDANT_VISIBILITY_MODIFIER`, but not on an override property, nor
  on an override of a member that inherits its visibility, and
  `USELESS_CALL_ON_NOT_NULL`, but not of a callable reference; and with
  `-Xuse-fir-experimental-checkers` `REDUNDANT_INTERPOLATION_PREFIX`.
  `@Suppress` silences a warning by its name, or all of them with
  `"warnings"`.

- Local `external` functions inside an interface's members when the
  interface's methods are JVM default methods (the default
  `-jvm-default`), which are reported as unsupported: kotlinc writes
  them as native interface methods, which the JVM refuses to load.
- Opt-in requirements (`@RequiresOptIn`, `@SubclassOptInRequired`) and
  `@Deprecated` are checked, the class path's read from its class files
  and metadata. Markers and deprecations with level `WARNING` (Java's
  `@Deprecated` and a Java source's javadoc `@deprecated` among them)
  give the warnings `OPT_IN_USAGE` and `DEPRECATION`. `@DeprecatedSinceKotlin` and
  `@SinceKotlin` are read as if their versions were reached: `kc` acts
  as the newest API version, as `kotlinc` does by default, and has no
  `-api-version`. A class path's typealias opts in to nothing, as the
  `@OptIn` it may have had in its source isn't in its class files: the
  markers of what it expands to are required of its uses.
- A class of the program with the name of a class of the class path
  takes its place in the class path's signatures and supertypes, as in
  K2. A class of the program's Java sources doesn't: the class path's
  signatures still name the class path's class of its name.
- A captured `var` is smart cast as in K2: not where a lambda or local
  function that may run later assigns it, nor where the outermost lambda
  entered since its declaration holds an assignment that isn't checked
  yet or assigns another type. In a loop, `kc` is stricter: any
  assignment in a lambda the loop holds counts, `x++` among them,
  whatever type it assigns.
- Two private classes or typealiases of one name in different files of
  a package aren't reported as a `CLASSIFIER_REDECLARATION`, and each
  file's uses resolve to its own, where K2 resolves them to the first
  file's and reports them as invisible.
- Two classes of one name on the class path, from different modules,
  are one class, the first, as in K2. A type in the other module's class
  files with another number of type arguments is a type of neither, so
  `A<Int>` isn't the first module's `A`. kc reads those type arguments only
  from Kotlin metadata, not from Java generic signatures: a Java class's
  method that names the other class keeps the first class.
- Builder inference isn't K2's PCLA: a builder's lambda is checked with
  its variables unfixed and then again with them fixed. A call in a
  nested builder that combines both builders' variables, as
  `select(this@outer.get(), get())`, is inferred with the inner
  builder's first, the outer one's in the system but left unfixed, and
  again with the outer one's. A fork of the constraint system over a
  builder's variables waits for its lambda's constraints. Of K2's
  fixation on demand, `kc` fixes a builder's variable for a member of a
  value of its type and for a nested call's lambda that takes the
  variable itself as a parameter, `it` of `flatMap { }` in `buildList`,
  not one in a type argument. A call's variable that captures an `out`
  projection of a builder's variable is reported as uninferred, as K2
  reports it, but kc infers its other captures of builders' variables,
  which K2 may not. An `if` or `when` whose branches mention a builder's
  variable joins them into its inference. A branch's generic call that
  only the builder's variable, not yet bounded, gives a type to, as
  `removeLast()` before `add(42)` in the other branch, takes the other
  branches' join as its bound, as K2's fixation of the join's variable
  first does; K2's own order of fixation isn't modeled otherwise, but
  for KT-58232: a builder call returned by the one-branch `when` a
  lambda ends with can't infer its variables, which are `Nothing`. A
  builder's lambda that casts its receiver to a
  star-projected subtype, `this as DerivedBuildee<*>`, infers the
  variable as `Unit`, where K2 infers a captured type. Of the errors a
  call adds to K2's shared constraint system, only one shape is carried
  to the calls after it: a generic extension called on an unresolved
  receiver with the lambda's `this`, `Unresolved().bar(this)`, after
  which the extension calls on the builder's receiver are inapplicable.

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
- An inline function of a class file built for a higher JVM target
  than `-jvm-target` (default 1.8), such as one `kotlinc` compiled for
  JVM 9 and later with `StringConcatFactory` string templates, is an
  error, as with `kotlinc` (`INLINE_FROM_HIGHER_PLATFORM`).

## Coroutines

- Two references of a suspend function type to the same suspend
  function aren't equal if they ignore its result, or are to an inline
  suspend function or to one that returns a value class of a reference
  or nullable type (not `Int` or `Long`).

## kotlinx.serialization (`-serialization`)

Not supported: generic value classes; interfaces annotated `@Serializable`
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
primitive types, `String`, enums, `@Serializable` classes (generic ones
too), objects and value classes, `UInt`, `ULong`, `UShort`, `UByte`,
interfaces (polymorphically), the class's type parameters, and `List`,
`Set`, `Map`, `HashSet`, `HashMap`, `Pair` and `Triple` of these, each
nullable too.

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

`kc` implements Metro 1.6.0's dependency graphs: `@DependencyGraph` and its factory, with `@Provides` and `@Includes` parameters; `@Inject` constructors and objects, `@Provides`, `@Binds`, qualifiers and `@SingleIn` scopes; requests as `() -> T`, `Provider<T>` and `Lazy<T>`; set and map multibindings, including `@ElementsIntoSet`, `@Multibinds` and wrapped map values; `@BindingContainer`; assisted injection; member injection; `@ContributesTo`, `@ContributesBinding`, `@ContributesIntoSet` and `@ContributesIntoMap` with `replaces` and `excludes`, from this module or the class path; and `@GraphExtension`s with their factories.

Not supported, and reported as such:

- custom annotations (the `custom-*` options), and options that change Metro's output, other than `enabled`, `contributes-as-inject` and `enable-function-providers`: among them `enable-derived-bindings` and a `companion-mode` other than `companion-object`;
- `@Provides` extension functions, and `@Provides` members of an
  interface or an abstract class outside its companion object;
- graphs with type parameters, and graph factories that are abstract classes;
- graph lifecycle callbacks: a graph that is a `CloseableGraph`, with its `@OnGraphInit` and `@OnGraphClose` functions;
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
  non-generic interfaces only when nothing else is shared), smart casts between a class type, `String` or a
  primitive type and an unrelated interface (`Int & A`), or of such an intersection to another (`C & A & B`),
  smart casts of a value of a type parameter's type to a class
  type, a primitive type or `String` (`String & T`, `Int & T`), identity or safe equality of two
  generic class types or a cast of one to the other (`Box<T> & Box<A>`),
  an `is` check or cast of one to a subclass of another instance
  (`B & A<T>` for a `B : A<Int>`), and incomparable upper bounds of a
  variable. Other smart casts are to the target type alone: a class type
  of a generic or Java type, a type parameter to another. A call on an
  intersection takes the first component with a member of the name
  whose parameters take its arguments, and a property read on one is
  the intersection of its types only over components of the same class;
  where a call's variable is constrained by an intersection whose
  components are different instances of one class, inference forks
  over them as K2 does, taking the first whose constraint system holds,
  but only over the call's own constraints, not its value's use, and
  with the cast's type first. A variable whose upper constraints don't meet its declared
  bounds, as `T` of `fun <T : Foo> foo(): T?` for a `Bar?`, is inferred
  from the constraints alone, where Kotlin infers `Foo & Bar`. A smart cast of a `T` to a value class is
  that type alone, which a generic call expecting a `T` takes only
  directly, as `listOf(o)`, not through another call, as `mapOf(o to o)`
  for an `o` of a value class. The common
  supertype of two builtin types is `Any`, or `Number` for numbers (for
  `Int` and `String`, Kotlin's is `Comparable<*> & Serializable`).
- Variables are fixed in K2's readiness order, but their constraints
  are bounds without K2's kinds and origins: an invariant argument is
  two bounds rather than an equality, and of the origins only declared
  bounds are told apart. A safe call passed to a generic call, whose
  selector K2 completes on its own, is part of the outer call's system,
  with its variables fixed first.
- A lambda's parameter types are fixed from the constraints known when
  the lambda is reached; K2 picks the order from the whole system, so it
  infers some combinations of lambdas `kc` can't. Of K2's ordering,
  `kc` runs the lambdas whose parameter types are known first, and a
  lambda that returns a lambda for a function type whose inputs aren't
  known yet last. As in K2, a lambda written before a nested call whose
  own lambda takes a type with variables in it (`mapValues`'
  `Map.Entry<K, V>`) runs first when only that lambda's result would
  infer its parameter's variable, which then is its declared bound. A
  lambda that declares
  all its parameters' types is checked on its own only when its
  variable has no function type among its bounds.
- The branches of an `if` or `when` aren't K2's synthetic call: a
  branch that is a lambda without its parameters' types takes them from
  the other branches only when the `if` has no expected type, and one
  takes an expected receiver as its first parameter only when it is a
  branch's value. Only a branch lambda written without parameters
  (`{ -> }`) of an `if` or `when` that is a generic call's argument is
  postponed with the call, as K2's synthetic call's arguments are.
- Conversions during inference are partial. A function value passed for
  a suspend function type with variables gives the converted type's
  constraint, and so does a generic call's result (`id(f)`) whose
  variable has one lower bound. A lambda passed for a variable takes
  its type from the variable's upper bound only when that is a suspend
  function type or a Java SAM interface; a lambda passed through another
  call (`id({ -> })`) isn't converted. A Java type parameter without
  constraints is inferred as its bound only when that bound is one
  class other than `Object`.
- `kc` has no captured types: a projection a variable stands for is
  approximated by its bounds where it is substituted. A variable that
  captures a projection of an argument is inferred as that projection,
  so a parameter of the argument's own or a supertype's type takes it
  (`Future<F>` for a `CompletableFuture<*>`), and a reified one is an
  error. A member's parameter whose type holds a projected argument
  takes `Nothing` there, except a lambda's function type, whose `out`
  projections are their bounds in its parameters and result, as K2
  approximates them; a function value passed for it can't return the
  capture, and a member's explicit type argument is checked against a
  bound with a projection approximated as such a parameter is. Two reads
  of one `Out<*>` value are the same type, where they are different
  captures in Kotlin: only an elvis's left call, inferred with the right
  side's type, sees a star of it as below its bound. A star for a
  parameter with a recursive bound reads as that bound with a star for
  the parameter (`Out3<*>` is an `Out3<Out3<*>>` for `Out3<out T :
  Out3<T>>`), as K2 approximates its capture. The capture of an
  `in` projection is checked against the variable's bounds only at the
  top of an argument's type. A property written through a projected
  receiver takes `Nothing` for a generic extension property's type
  parameter its receiver projects out, and for an invariant argument of
  a member property's type that a star captures, checked after the
  value is inferred against the read type; so does a Java field or a
  Java setter's parameter. A Java setter called as a method
  (`setWrapper(w)` on a `Container<*>`) doesn't, and an invariant
  argument that an `in` projection captures takes the projection.
- Of K2's captured types, `kc` has only those of reading an explicit
  backing field through a stable receiver whose type projects the
  field's type parameters, `o.num` for an `o: C<*>`: the receiver is
  smart cast to its type with the projections captured, whose members
  then take the field's value, `o.consume(o.num)`.
- A lambda's result chooses between overloads of the program's
  functions only when one of them is marked
  `@OverloadResolutionByLambdaReturnType`, as in K2, but between
  library functions' whether or not they are, since `kc` doesn't read
  the mark from class files. `@HidesMembers` is read only on the
  program's functions; the standard library's `forEach` hides only
  members that just Java's collection interfaces declare.
- `@OnlyInputTypes` (`contains`, `get`, `assertEquals`) is checked only
  when the types a call's variable is constrained by have no captured
  types (star projections) and no type parameters out of scope where
  the call is, and the inferred type isn't an intersection's component.
- Of the Java collection members K2 gives Kotlin's signatures, `kc` maps
  `contains`, `remove`, `containsAll`, `removeAll`, `retainAll`,
  `indexOf`, `lastIndexOf`, and a map's `containsKey`, `containsValue`,
  `get` and both `remove`s; others, such as `getOrDefault`, keep Java's,
  though named arguments take Kotlin's parameter names.
- K2's predefined nullability of JDK members is applied to the members
  of the classes it names, not to their overrides in other classes:
  `Map`'s `compute` takes a function that may return null, `HashMap`'s
  keeps Java's platform types.
- A Java class's `@kotlin.jvm.PurelyImplements` is honoured only when it
  names one of Kotlin's collection types (`MutableList`, `Set`, `Map`
  and the others), the only ones K2's JDK classes (`ArrayList`,
  `HashSet`, `HashMap` and others) purely implement too.
- A nullable argument for a Java method's `T!` makes `T` nullable,
  except in an in-projected type argument, where kc's substituted
  explicit type arguments stand for K2's equality constraints. Even
  where the call's expected type would have `T` not null:
  `val s: Set<String> = java.util.HashSet(setOf(null))` is an error.
- A type written as a Java class that Kotlin maps to a built-in type,
  as `java.lang.CharSequence` or `java.lang.Comparable`, is the
  built-in type: `kotlinc` keeps the Java class, so that a
  `java.lang.Comparable<*>` isn't a `kotlin.Comparable<*>`. Only a
  smart cast by `is java.lang.CharSequence` keeps the Java class's
  `length()` method callable. So is a type written as one of the
  classes `kotlin.jvm.functions.FunctionN`, the function type, but where
  a lambda, an anonymous function or a callable reference initializes a
  property or a variable of that type, which is an error, as with
  `kotlinc`: one passed for a parameter of that type is taken.
- jSpecify's `@Nullable` and `@NonNull` are read only at the top of a
  Java method's, parameter's or field's type, and `@NullMarked` and
  `@NullUnmarked` only on the classes and methods of the program's Java
  sources, not on packages, modules or class files.
- Of Java members, only those of the program's Java sources count
  toward an inherited member that must be overridden because it has
  several implementations; a class-path Java class's do not.
- Java sources compiled with the program see the member classes a
  Kotlin class inherits only through supertypes written without type
  arguments: a Java subclass of `class K : Base<String>()` that names
  `Inner`, declared in `Base<T>`, leaves the name unresolved in the
  signatures `kc` reads, as the inner class's `T` would be lost.
- Without an expected type, a reference to a vararg function, or to a
  generic function whose type parameters its receiver's type doesn't
  give (`Box<Int>::first` for `fun <T> Box<T>.first()` is fine), is an
  error.
- Where annotations are written isn't checked for Java annotations
  without `@Target` or from Java sources, nor whether they repeat on a
  property or with a use-site target, nor, for any annotation, for
  use-site targets on expressions other than `@all:`, nor across a typealias's expansion
  (`@A T` where `typealias T = @A Int`). The arguments of a
  library's `SOURCE` annotation aren't checked, so they need not be
  constants.

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
- A reference to a library function that `kc` compiles as built in,
  as `String::toInt`, names the receiver's class (`String`) as its
  owner, where `kotlinc`'s names the library's file facade
  (`StringsKt`).
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
- Class files for a `-jvm-target` above 1.8 are those for 1.8 with a
  higher version: no `PermittedSubclasses` attributes, which `kotlinc`
  writes for `-jvm-target 17` and later, and string templates are built
  with `StringBuilder`, not `kotlinc`'s `StringConcatFactory` of 9 and
  later. `InnerClasses` entries are only
  for the file's own nested classes, not for those a class refers to.
- A value class's inherited members are called on the box, and `==`
  boxes both values and calls `equals` rather than `equals-impl0`.
- `@JvmExposeBoxed` is checked as `kotlinc` checks it, but `kc` doesn't
  write the boxed overloads it asks for: Java sees only the mangled or
  unboxed declarations.
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
- A generic `@Serializable` class's `$serializer` has a public primary
  constructor that takes its type arguments' serializers. kotlinc's has
  a private constructor without parameters, which builds the descriptor,
  and a public secondary constructor that calls it, which has the same
  signature as `kc`'s.
