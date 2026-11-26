# Unsupported Kotlin

`kc` compiles Kotlin 2 for the JVM with `kotlinc`'s ABI (see [The JVM
ABI](design.md#the-jvm-abi)). This lists what it doesn't compile, or
compiles only in part, and then, in [Differences from
kotlinc](#differences-from-kotlinc), where what it compiles differs from
`kotlinc`'s output. What isn't listed is meant to work as with `kotlinc`.
[Conformance](conformance.md) measures how far that holds on `kotlinc`'s
own tests.

## The language

- Warnings: `kc` reports `kotlinc`'s warnings by their names and
  messages, but only where the types make them certain, so that
  `-Werror` doesn't reject what `kotlinc` compiles. `@Suppress`
  silences a warning by its name, or all of them with `"warnings"`.
  These warnings of `kotlinc` 2.4.21 aren't reported:
  - Those of Java's nullability and type annotations and Java
    declarations: `TYPE_MISMATCH_BASED_ON_JAVA_ANNOTATIONS`,
    `RECEIVER_NULLABILITY_MISMATCH_BASED_ON_JAVA_ANNOTATIONS`,
    `RECEIVER_MUTABILITY_MISMATCH_BASED_ON_JAVA_ANNOTATIONS`,
    `NULLABILITY_MISMATCH_BASED_ON_EXPLICIT_TYPE_ARGUMENTS_FOR_JAVA`,
    `UNEXHAUSTIVE_WHEN_BASED_ON_JAVA_ANNOTATIONS`,
    `UPPER_BOUND_VIOLATED_BASED_ON_JAVA_ANNOTATIONS`,
    `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION_BASED_ON_JAVA_ANNOTATIONS`,
    `TYPE_MISMATCH_WHEN_FLEXIBILITY_CHANGES`,
    `EXPANSIVE_INHERITANCE_IN_JAVA`, `FINITE_BOUNDS_VIOLATION_IN_JAVA`,
    `WRONG_TYPE_FOR_JAVA_OVERRIDE`,
    `SYNTHETIC_PROPERTY_WITHOUT_JAVA_ORIGIN`,
    `SYNCHRONIZED_BLOCK_ON_JAVA_VALUE_BASED_CLASS` and
    `IDENTITY_SENSITIVE_OPERATIONS_WITH_VALUE_TYPE`.
  - Those of classes missing from the class path:
    `MISSING_DEPENDENCY_SUPERCLASS_WARNING`,
    `MISSING_DEPENDENCY_CLASS_IN_EXPRESSION_TYPE`,
    `MISSING_DEPENDENCY_CLASS_IN_LAMBDA_PARAMETER`,
    `MISSING_DEPENDENCY_CLASS_IN_LAMBDA_RECEIVER` and
    `MISSING_DEPENDENCY_CLASS_IN_TYPEALIAS`.
  - Those of multiplatform declarations:
    `ACTUAL_ANNOTATIONS_NOT_MATCH_EXPECT`,
    `ACTUAL_IGNORABILITY_NOT_MATCH_EXPECT` and
    `EXPECT_ACTUAL_CLASSIFIERS_ARE_IN_BETA_WARNING`.
  - Those of annotations: `WRONG_ANNOTATION_TARGET_WARNING`,
    `INAPPLICABLE_TARGET_ON_PROPERTY_WARNING`,
    `ANNOTATION_WILL_BE_APPLIED_ALSO_TO_PROPERTY_OR_FIELD`,
    `REDUNDANT_ANNOTATION`, `REPEATED_ANNOTATION_WARNING`,
    `INAPPLICABLE_JVM_FIELD_WARNING` and
    `OPT_IN_MARKER_ON_OVERRIDE_WARNING`.
  - Those of modifiers: `DEPRECATED_MODIFIER_PAIR`,
    `DEPRECATED_MODIFIER_CONTAINING_DECLARATION`,
    `INAPPLICABLE_OPERATOR_MODIFIER_WARNING`,
    `CANNOT_CHANGE_ACCESS_PRIVILEGE_WARNING` and
    `CANNOT_INFER_VISIBILITY_WARNING`.
  - Those of deprecated forms: `DEPRECATED_ACCESS_TO_ENTRIES_PROPERTY`,
    `DEPRECATED_ACCESS_TO_ENTRIES_AS_QUALIFIER`,
    `DEPRECATED_ACCESS_TO_ENTRY_PROPERTY_FROM_ENUM`,
    `DEPRECATED_ACCESS_TO_ENUM_ENTRY_COMPANION_PROPERTY`,
    `DEPRECATED_ACCESS_TO_ENUM_ENTRY_PROPERTY_AS_REFERENCE`,
    `DEPRECATED_SMARTCAST_ON_DELEGATED_PROPERTY`,
    `DEPRECATION_ERROR_MIGRATION_PERIOD_WARNING`,
    `VERSION_REQUIREMENT_DEPRECATION`, `ROOT_IDE_PACKAGE_DEPRECATED`,
    `NON_PUBLIC_CALL_FROM_PUBLIC_INLINE_DEPRECATION`,
    `EXPOSED_TYPE_PARAMETER_BOUND_DEPRECATION_WARNING`,
    `TYPE_PARAMETER_AS_REIFIED_DEPRECATION_WARNING`,
    `TYPE_INTERSECTION_AS_REIFIED_DEPRECATION_WARNING`, and
    `UPPER_BOUND_VIOLATED_DEPRECATION_WARNING` but of a typealias's
    constructor call (see below).
    Of the diagnostics that are a `_WARNING` until a language version
    makes them an `_ERROR`, `kc` reports only
    `IMPOSSIBLE_IS_CHECK_WARNING`,
    `IMPOSSIBLE_IS_CHECK_RELYING_ON_NULL_WARNING`,
    `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING`,
    `UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION_DEPRECATION_WARNING`
    and `UPPER_BOUND_VIOLATED_IN_LHS_OF_CLASS_LITERAL_WARNING`, and, in
    part, `INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING` and
    `UPPER_BOUND_VIOLATED_DEPRECATION_WARNING` (see below).
  - Those of types and type arguments:
    `MUTABLE_PROPERTY_WITH_CAPTURED_TYPE`,
    `TYPE_ARGUMENTS_NOT_ALLOWED_WARNING`,
    `TYPE_ARGUMENTS_NOT_ALLOWED_IN_PACKAGE_QUALIFIER_WARNING`,
    `EXPLICIT_TYPE_ARGUMENTS_IN_PROPERTY_ACCESS_WARNING`,
    `CONFLICTING_PROJECTION_IN_CALLABLE_REFERENCE_WARNING`,
    `LOCAL_VARIABLE_WITH_TYPE_PARAMETERS_WARNING`,
    `WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_GET_CLASS_WARNING`, and of class
    literals: `CLASS_LITERAL_LHS_NOT_A_CLASS_WARNING`,
    `EXPRESSION_OF_NULLABLE_TYPE_IN_CLASS_LITERAL_LHS_WARNING` and
    `UNSUPPORTED_ARRAY_OF_NOTHING_IN_CLASS_LITERAL_LHS`.
  - Those of initialization and inline functions:
    `MUST_BE_INITIALIZED_WARNING`,
    `MUST_BE_INITIALIZED_OR_BE_ABSTRACT_WARNING`,
    `MUST_BE_INITIALIZED_OR_BE_FINAL_WARNING`,
    `MUST_BE_INITIALIZED_OR_FINAL_OR_ABSTRACT_WARNING`,
    `INITIALIZATION_BEFORE_DECLARATION_WARNING`,
    `NOT_YET_SUPPORTED_IN_INLINE_WARNING` and
    `INVISIBLE_REFERENCE_WARNING`.
  - Others: `MISSING_BRANCH_FOR_NON_ABSTRACT_SEALED_CLASS`,
    `DESTRUCTURING_SHORT_FORM_NAME_MISMATCH`,
    `DESTRUCTURING_SHORT_FORM_OF_NON_DATA_CLASS`,
    `DESTRUCTURING_SHORT_FORM_UNDERSCORE`,
    `UNNAMED_PROPERTY_WITH_IMPLICIT_IGNORABLE_TYPE`,
    `RESOLVED_TO_UNDERSCORE_NAMED_CATCH_PARAMETER`,
    `REDUNDANT_LABEL_WARNING`, `REDUNDANT_RETURN`,
    `ESCAPING_CAPTURED_VARIABLE`, `WRAPPED_LHS_IN_ASSIGNMENT_WARNING`,
    `INEFFICIENT_EQUALS_OVERRIDING_IN_VALUE_CLASS`,
    `K_SUSPEND_FUNCTION_TYPE_OF_DANGEROUSLY_LARGE_ARITY`, and those of
    the return value checker: `RETURN_VALUE_NOT_USED`,
    `RETURN_VALUE_NOT_USED_COERCION` and
    `OVERRIDING_IGNORABLE_WITH_MUST_USE`.

  Of a program with a syntax error, which `kc` checks without the
  statements and declarations that have the errors, only the warnings
  that what is left out can't be the reason for are reported: those
  judged by the syntax and modifiers of what is kept, by the types of a
  declaration's header, or by two declarations that are both kept, such
  as `REDUNDANT_MODIFIER` and `FINAL_UPPER_BOUND`; not those of what
  isn't used or reached, of an expression's type, of what a name
  resolves to, or of overrides. `kotlinc`'s command line prints no
  warnings at all with an error.

  These are reported only in part:
  - The null checks, such as `UNNECESSARY_SAFE_CALL`,
    `SENSELESS_COMPARISON` and `USELESS_ELVIS`, never of a platform
    type, a type parameter or a type `kc` couldn't resolve.
  - Type checks and casts, such as `USELESS_IS_CHECK`,
    `IMPOSSIBLE_IS_CHECK_WARNING`, `USELESS_CAST` and
    `CAST_NEVER_SUCCEEDS`, are judged as `kotlinc` judges them, by the
    declared type and then by its smart casts, but never where a type
    `kc` couldn't resolve is involved, and a platform operand only by
    its nullable upper bound, for `USELESS_IS_CHECK` and `USELESS_CAST`.
  - `UNCHECKED_CAST` only of a cast to a type parameter that isn't
    reified (of another type parameter's value only where it isn't
    smart cast), to a generic type from `Any` or a star-projected
    supertype, to a function type from another of its class, or to a
    generic type whose type arguments its operand's type, smart casts
    included, doesn't tell (a primitive or a `String` only where it
    isn't smart cast), judged as `kotlinc` does but only where
    that type's arguments map to the target class's type parameters
    one by one, without projections, and not of a lambda or a
    reference to an interface; of a platform operand only where its
    type arguments are all stars, as a cast of `Class.forName(name)`.
  - `PARAMETER_NAME_CHANGED_ON_OVERRIDE`,
    `DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES` and
    `DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE` compare parameter names
    only where they are stable, of the program's and Kotlin libraries'
    functions and the built-in classes' members (`Any`, `CharSequence`,
    `Comparable` and the collections), not Java methods, and the last
    two only of the program's Kotlin members.
  - `PLATFORM_CLASS_MAPPED_TO_KOTLIN` where the Java class is named,
    not through a typealias, and at a call that infers it as a type
    argument only for a boxed primitive's class, as
    `java.lang.Integer::class.java`.
  - `NO_TAIL_CALLS_FOUND` not where a recursive call in tail position
    may be a tail call to `kotlinc`, as one on another receiver.
  - `UNUSED_EXPRESSION` never of property reads, calls or operators.
  - `EXTENSION_SHADOWED_BY_MEMBER` and
    `EXTENSION_FUNCTION_SHADOWED_BY_MEMBER_PROPERTY_WITH_INVOKE` by the
    members of the program's classes and of the class path's Kotlin
    classes, not of Java classes or built-in types such as `List` and
    `Int`.
  - `REDUNDANT_ELSE_IN_WHEN` only of a `when` over a Boolean, an enum
    or a sealed class, or one exhaustive through an unguarded `is`
    check of a supertype of the subject's type; not over a type
    parameter's value, a Java enum or a Java sealed class.
  - `INCOMPATIBLE_ENUM_COMPARISON` and
    `FORBIDDEN_IDENTITY_EQUALS_WARNING` not where a platform type is
    involved.
  - `EQUALITY_NOT_APPLICABLE_WARNING` of smart-cast values, not of
    enums nor an identity comparison of a value without identity, and
    of unrelated types that K1 let both have subtypes, as an interface
    and a class compared with `===`.
  - `INCOMPATIBLE_TYPES_WARNING` only of a `when` whose subject is a
    smart-cast variable.
  - `UPPER_BOUND_VIOLATED_DEPRECATION_WARNING` only of a typealias's
    constructor call that infers the typealias's type arguments, of a
    class in its type's type arguments.
  - `INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING` and
    `INFERRED_TYPE_VARIABLE_INTO_POSSIBLE_EMPTY_INTERSECTION` of a call's
    type argument that `kc` infers as an intersection, or that only
    upper constraints and its declared bounds give, and of a delegated
    property's `getValue`; not of a callable reference's vararg
    adaptation or a lambda's last expression after a bare `return`.
  - `ERROR_SUPPRESSION`, reported, as `kotlinc` 2.4.21 does, for every
    error name in a `@Suppress`, but not in one on a type or a
    supertype.
  - `DSL_MARKER_APPLIED_TO_WRONG_TARGET` not on a lambda.
  - `REDUNDANT_EXPLICIT_BACKING_FIELD` not of a field the property
    can't have.
  - `LEAKED_IN_PLACE_LAMBDA` and `WRONG_INVOCATION_KIND` not of a
    parameter used in a callable reference, a safe call or a context
    argument, nor the invocation kind where its calls are in a loop
    that jumps, a lambda that jumps, or a `when` without `else` that
    isn't exhaustive, or in a function that never returns normally.
  - `MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING`
    not of a class that doesn't override the function.
  - `UNNECESSARY_LATEINIT` not where an object expression or a local
    class reads the property, nor of a type parameter's type, nor for
    constructors whose `this(...)` calls loop.
  - `WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING` and
    `WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING` not
    through a typealias, nor of an inner class of a generic nested
    class, nor of a local class inside a generic class.
  - `ILLEGAL_TYPE_ARGUMENT_FOR_VARARG_PARAMETER_WARNING` not of a
    constructor call.
  - `CONTEXTUAL_OVERLOAD_SHADOWED` not where a platform type or a type
    `kc` couldn't resolve is involved, nor between overloads with more
    than sixteen pairs of context parameters to compare.
  - `REDUNDANT_ANNOTATION_TARGET` not of an annotation in a bracketed
    list, as `@param:[A B]`.
  - `INCOMPATIBLE_ANNOTATION_TARGETS` only where Java's `@Target` names
    the entries of `ElementType`, not constants of its own.
  - `DEPRECATED_MODIFIER_FOR_TARGET` only of `external` on an enum entry
    or a constructor's property, and `REDUNDANT_MODIFIER_FOR_TARGET`
    only of an `open interface`.
  - `OPERATOR_RENAMED_ON_IMPORT` of top-level functions and objects'
    members, the program's and the class path's Kotlin ones.
  - `CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING` only of an inherited var's
    setter.
  - `INFERRED_INVISIBLE_RETURN_TYPE_WARNING`,
    `INFERRED_INVISIBLE_WHEN_TYPE_WARNING`,
    `INFERRED_INVISIBLE_REIFIED_TYPE_ARGUMENT_WARNING` and
    `INFERRED_INVISIBLE_VARARG_TYPE_ARGUMENT_WARNING` only of a private
    top-level class of another file or a package-private Java class of
    another package, or a class nested in one, not of a private or
    protected nested class, nor of an `if` or `when` that an expected
    type gives a type.
  - `JAVA_CLASS_ON_COMPANION` not through an alias of a built-in type.
  - `IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT` not of a
    delegated property.
  - `DANGEROUS_CHARACTERS` not of a function type's parameter names,
    nor of a destructuring `for` loop's.
  - With `-Wextra`: `UNREACHABLE_CODE` only after a statement that
    always jumps, in the right operand of `true ||` and `false &&` and
    in the body of `while (false)`, not after a loop whose only `break`
    is in such an operand, nor the dead code of a class's
    initialization order;
    `ASSIGNED_VALUE_IS_NEVER_READ` and
    `VARIABLE_INITIALIZER_IS_REDUNDANT` only where no statement that
    may read the value comes before it is overwritten or out of scope,
    through `if`, `when`, `try` and loops without `break` or
    `continue`, a catch taken as entered after any statement of the
    `try`, and, for a variable a lambda, a local function or a local
    class reads, only before the first of them and outside loops, and
    in a lambda that only assigns it, not inline, only where nothing
    reads it after the lambda; `UNUSED_ANONYMOUS_PARAMETER` not
    of a parameter whose name the lambda's body uses for another
    variable; `REDUNDANT_VISIBILITY_MODIFIER` not on an override
    `var` whose modifier may widen a setter it inherits, an override
    property's accessors, nor on an override of a member that inherits
    its visibility; `USELESS_CALL_ON_NOT_NULL` not of a callable
    reference; and `CAN_BE_VAL_DELAYED_INITIALIZATION` and
    `CAN_BE_VAL_LATEINIT` not of a `var` assigned in a loop that runs
    once, `do { … } while (false)`.

- Local `external` functions inside an interface's members when the
  interface's methods are JVM default methods (the default
  `-jvm-default`), which are reported as unsupported: kotlinc writes
  them as native interface methods, which the JVM refuses to load.
- Opt-in requirements (`@RequiresOptIn`, `@SubclassOptInRequired`) and
  `@Deprecated` are checked, the class path's read from its class files
  and metadata. Markers and deprecations with level `WARNING` (Java's
  `@Deprecated` and a Java source's javadoc `@deprecated` among them)
  give the warnings `OPT_IN_USAGE` and `DEPRECATION`. `@DeprecatedSinceKotlin`'s
  versions are judged against API version 2.4, `kotlinc` 2.4's default,
  and `@SinceKotlin`'s are read as if reached; `kc` has no
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
  yet or assigns another type, nor where an assignment that may run
  while the lambda does, in a loop around it among them, does; `x++` and
  `x--` aren't assignments there. A lambda that may run more than once
  is a loop: it drops the smart casts of the vars it assigns with `=` at
  its start.
- Two classes of one name on the class path, from different modules,
  are one class, the first, as in K2. A type in the other module's class
  files with another number of type arguments is a type of neither, so
  `A<Int>` isn't the first module's `A`. kc reads those type arguments only
  from Kotlin metadata, not from Java generic signatures: a Java class's
  method that names the other class keeps the first class.
- Builder inference follows the rules of K2's PCLA: a builder's lambda
  is checked once, with the builder's variables unfixed, on the
  constraint system that the builder's call shares with the calls in
  the lambda and with the builders around and in it. A call that
  mentions the variables is postponed as K2's is, a variable is fixed on
  demand, for a member of a value of its type or a type written without
  its arguments in an `is` or `as`, only from the proper constraints it
  has at that statement, and the errors of this check are the lambda's:
  a member of a value whose variable can't be fixed yet is unresolved,
  whatever the statements after it infer, and doesn't keep the
  variables from being inferred. The inferred types are then written
  into the lambda's tree. What differs
  from K2:
  - A call that fails K2's shared constraint system, a generic
    extension called on an unresolved receiver with a lambda or the
    lambda's `this` (`Unresolved().apply { }`), fails the calls after
    it as K2 does, but leaves the builder's variables uninferred even
    where a call before it gave them a type, and words the errors of
    the members of a value whose type wasn't inferred differently.
  - A variable that is a component of a smart cast's intersection,
    `OTv & A` after `x is A` or an assignment, is fixed for a member as
    K2 fixes it, but where nothing bounds it yet K2 reports that the
    receiver's type hasn't been inferred, and kc reports nothing there.
    One that a projection captures is fixed too, where K2 leaves its
    member unresolved (KT-71843). kc fixes one for `++x[i]` and
    `x[i]++` too, which K2 doesn't (KT-71778).
  - A call of a Java generic method between the variables of two
    nested builders, `select(get(), Test.foo(this@outer.getInv()))`, is
    accepted where K2 reports its argument.
  - A lambda that a builder's lambda returns before its end, `if (b)
    return@build { arg -> arg.length }`, an overload chosen by a
    lambda's result type with a builder's variable in it, which K2
    finds ambiguous where only other variables bound that variable, and
    the positions of K2's "cannot infer"
    on a parameter or a type argument (`CANNOT_INFER_PARAMETER_TYPE`)
    in a lambda with errors are kc's own.
  - A fork of the constraint system over a builder's variables waits
    for its lambda's constraints. A call's variable that captures an
    `out` projection of a builder's variable is reported as uninferred,
    as K2 reports it, but kc infers its other captures of builders'
    variables, which K2 may not.
  - An `if` or `when` whose branches mention a builder's variable joins
    them into its inference. A branch's generic call that only the
    builder's variable, not yet bounded, gives a type to, as
    `removeLast()` before `add(42)` in the other branch, takes the
    other branches' join as its bound, as K2's fixation of the join's
    variable first does; K2's own order of fixation isn't modeled
    otherwise, but for KT-58232: a builder call returned by the
    one-branch `when` a lambda ends with can't infer its variables,
    which are `Nothing`.
  - A builder's lambda that casts its receiver to a star-projected
    subtype, `this as DerivedBuildee<*>`, infers the variable as
    `Unit`, where K2 infers a captured type.
  - The right side of an elvis gives the call on its left its
    variables before that call's lambda is analyzed, as K2's `K` of
    `elvis(x: K?, y: K)` does, only in the initializer of a local
    declared without a type: `val x = find { it.length } ?: ""`.

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

- Unsigned literals (`1u`) are an error, as with `kotlinc`
  (`UNSIGNED_LITERAL_WITHOUT_DECLARATIONS_ON_CLASSPATH`): the unsigned
  types are the standard library's.

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
(`by`); superclasses from other modules; properties in the class body
that are set in an `init` block rather than initialized, or have
accessors of their own; some ways of naming serializers of properties (a
generic class in `@Serializable(with = ...)` on a property or in
`@file:UseSerializers`); `@KeepGeneratedSerializer` and
`@MetaSerializable`; the annotations that formats read (`@JsonNames`,
`@ProtoNumber`...); `Array<*>`, and the JDK's types without
`@UseSerializers` or `@Contextual` (`java.util.Date`...). Properties
can be of the primitive types, `String`, `Unit`, enums, `@Serializable`
classes (generic ones too), objects and value classes, `UInt`, `ULong`,
`UShort`, `UByte`, `Duration`, `Instant`, `Uuid`, interfaces
(polymorphically), the class's type parameters, the primitive and
unsigned arrays, and `Array`, `List`, `Set`, `Map`, `HashSet`,
`HashMap`, `Pair` and `Triple` of these, each nullable too; of types
`@Contextual` or `@Polymorphic` mark, on the property or its type, and of
those a file's `@UseSerializers` and `@UseContextualSerialization`
name.

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
  whose parameters take its arguments (of those without parameters, the
  one with the most specific result, a Kotlin one over a Java method's
  platform type), and a property read on one is
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
  the other branches only when the `if` has no expected type. A lambda
  takes an expected receiver and context parameters as its first
  parameters, as in K2, only where it is a branch's value, the
  initializer of a property or variable of a declared type, or an
  argument for a parameter whose type is a type parameter. Only a branch lambda written without parameters
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
  error. As in K2, a call's arguments can't give such a variable a
  second capture, or a type below it other than `Nothing`: `cp(x, y)`
  for `fun <T> cp(a: MutableList<T>, b: MutableList<T>)` and two
  `MutableList<out Number>`s, or `foo(m)` for a `Map<*, *>` passed for
  a `Map<T, T>`, is an error; but only where the projections are the
  arguments' own type arguments, not those of a receiver or nested
  deeper in a parameter's type. A member's parameter whose type holds a projected argument
  takes `Nothing` there, except a lambda's function type (see below);
  a function value passed for it can't return the capture, and a
  member's type parameter, explicit or inferred, is checked against a
  bound with a projection approximated as such a parameter is (`E :
  Out<T>` of an `A<out CharSequence>` is bounded by `Out<Nothing>`). Two reads
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
  Java setter's parameter. A call's parameter, and a Java synthetic
  setter's, takes `Nothing` for an invariant argument that a star or an
  `in` projection captures (`setW(w)` of a `KC<in String>`); a Kotlin
  property's assignment keeps the projection, as K2 approximates it.
- Of K2's captured types, `kc` has only those of reading an explicit
  backing field through a stable receiver whose type projects the
  field's type parameters, `o.num` for an `o: C<*>`: the receiver is
  smart cast to its type with the projections captured, whose members
  then take the field's value, `o.consume(o.num)`; and those of a
  receiver's `out` projection, not of an intersection, in the function
  type of a member's parameter that a lambda is passed for: the lambda
  takes the capture, which reads as the projection's type, and returns
  only the capture or `Nothing` (`a.foo { "" }` is a mismatch for
  `foo(block: () -> E)` of an `A<out CharSequence>`, `x.foo { it }`
  isn't for `foo(f: (E) -> E)`). A variable of another lambda's input
  types fixed to the capture takes the projection's type, as K2
  approximates it; other variables keep the capture.
- A lambda's result chooses between overloads of the program's
  functions only when one of them is marked
  `@OverloadResolutionByLambdaReturnType`, as in K2, but between
  library functions' whether or not they are: `kc` reads the mark from
  class files only to choose the one candidate that isn't marked when
  the lambda's result fits none. `@HidesMembers` is read only on the
  program's functions; the standard library's `forEach` hides only
  members that just Java's collection interfaces declare.
- `@OnlyInputTypes` (`contains`, `get`, `assertEquals`) is checked only
  when the types a call's variable is constrained by have no captured
  types (star projections) and no type parameters out of scope where
  the call is, and the inferred type isn't an intersection's component.
- Of the Java collection members K2 gives Kotlin's signatures, `kc` maps
  `contains`, `remove`, `containsAll`, `removeAll`, `retainAll`,
  `indexOf`, `lastIndexOf`, and a map's `containsKey`, `containsValue`,
  `get`, `getOrDefault` and both `remove`s; others keep Java's, though
  named arguments take Kotlin's parameter names. Only the collection
  class's own methods are mapped: one it inherits from a superclass
  that isn't a collection, as `remove(Object)` of
  `class A extends AImpl implements List<String>`, keeps Java's.
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
- Nullability annotations, jSpecify's `@Nullable` and `@NonNull` among
  them, are read only at the top of a Java method's, parameter's or
  field's type, and of a type parameter's bound: there, from Java sources
  and class files, a not-null one on a type parameter's only bound, `T
  extends @NotNull Object`, makes the bound and the type parameter's uses
  in its class's or method's members non-null, as in K2. One on a bound
  among several isn't read, nor are those inside a type, such as
  `JavaBox<@NotNull T>`. `@NullMarked` and `@NullUnmarked` are read only
  on the classes and methods of the program's Java sources, not on
  packages, modules or class files. A nullable one on a type parameter's
  bound, `T extends @Nullable Object`, changes nothing: K2 rejects
  `n2<String>(null)` for a Java `<T extends @Nullable Object> void n2(T)`,
  which `kc` compiles.
- A `null` passed for a Java parameter of a flexible type variable,
  `JavaBox(null)`, whose variable is then equal to a type parameter bound
  `Any`, as in `nn(JavaBox(null))` for a Java `<T extends @NotNull Object>
  void nn(JavaBox<T>)`, infers the type parameter as `Any` where K2 can't
  infer it: `kc` compiles the call.
- Of Java members, only those of the program's Java sources count
  toward an inherited member that must be overridden because it has
  several implementations; a class-path Java class's do not.
- Java sources compiled with the program see the member classes a
  Kotlin class inherits only through supertypes written without type
  arguments: a Java subclass of `class K : Base<String>()` that names
  `Inner`, declared in `Base<T>`, leaves the name unresolved in the
  signatures `kc` reads, as the inner class's `T` would be lost.
- Without an expected type, a reference to a generic function whose
  type parameters its receiver's type doesn't give (`Box<Int>::first`
  for `fun <T> Box<T>.first()` is fine) is an error.
- A value of an error type has members without errors where K2
  reports some of them unresolved: an element of an array of such a
  type, or a property whose delegate has no `getValue`. (The members
  of such a value smart-cast to non-null, or of an inner class
  constructed without an outer instance, are reported.)
- An overload whose parameter's type `kc` couldn't resolve isn't a
  candidate for a call, which K2 finds ambiguous between it and the
  others: `f1(l)` for an `f1(l: Unresolved<T>)` and an
  `f1(c: Collection<T>)` calls the latter.
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
  higher version, and from 17 on `PermittedSubclasses` attributes, as
  `kotlinc` writes them: string templates are built with
  `StringBuilder`, not `kotlinc`'s `StringConcatFactory` of 9 and
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
- Annotations on type parameters and types aren't written.

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
