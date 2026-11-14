# Plan: type inference closer to K2

This plan covers bringing `kc`'s type inference closer to K2's, to cut
the failures in the [diagnostics suite](../conformance.md). It is a
plan, not a description of `kc`: nothing in it is implemented yet. The
numbers are from `bench/results/diagnostics.tsv` at commit `62c0fcb`,
reproduced with `bench/kotlin-tests.sh diagnostics` (0 regressions, 0
fixed against the baseline).

The diagnostics suite has 598 failing tests: 420 *accepts invalid* and
178 *rejects valid*. Inference is the root cause of 99 of them (37
accepts invalid, 62 rejects valid), and a contributing cause of 7 more
in overload resolution. It also causes most of the errors `kc` reports
on lines where `kotlinc` reports nothing: 692 of them, in 250 tests,
are `type mismatch: inferred type is _ but _ was expected`, `cannot
infer type for type parameter` or `cannot infer a type for this
parameter`, and 242 of those are in builder-inference tests.

The work maps onto four items of [unsupported.md](../unsupported.md):
builder inference that forks the constraint system or nests builders,
intersection types, the order in which lambdas' parameter types are
fixed, and `@OnlyInputTypes`. It also needs three things that
unsupported.md doesn't list: K2's common supertype, constraints from
platform types, and captured types as types of their own.

## Contents

1. [The failing tests](#1-the-failing-tests), grouped by root cause.
2. [How kc infers types](#2-how-kc-infers-types) today.
3. [The gap against K2](#3-the-gap-against-k2).
4. [Milestones](#4-milestones), cheap ones first.
5. [Validation](#5-validation).
6. [unsupported.md](#6-unsupportedmd): what each milestone lets us delete or rewrite.

## 1. The failing tests

The set was re-derived from the failing tests in
`bench/results/diagnostics.tsv` and the test sources, not from their
directories. Each failing test whose key or first error is about types
(type mismatches, inference errors, bounds, ambiguities, intersections,
captured types) was read and given the root cause that its first error,
or the error it lacks, comes from. Tests whose failure only looks like
inference were left out: for example `tests/j+k/collectionOverrides/*`,
where the Java `contains(Object)` member should be hidden behind
Kotlin's `contains(E)`, `platformTypes/nullabilityWarnings/*`, which
need Java nullability annotations, and `generics/capturedParameters/*`,
which is about local classes.

The errors on unmarked lines come from a run of the harness that also
wrote each test's errors on lines without expected diagnostics (a
temporary local change; milestone 0 makes it a flag).

| Group | Root cause | Accepts invalid | Rejects valid | Total |
|:---|:---|---:|---:|---:|
| A | `@OnlyInputTypes` not checked for literals, projections or intersections | 6 | 0 | 6 |
| B | Empty intersections: an error where K2 warns, and no inapplicable candidate where K2 has one | 0 | 6 | 6 |
| C | Common supertypes and intersection types | 1 | 11 | 12 |
| D | Captured types | 3 | 9 | 12 |
| E | Constraint incorporation and variable fixation order | 1 | 9 | 10 |
| F | Postponed arguments: lambdas, and `if`/`when`/elvis as calls | 1 | 9 | 10 |
| G | `@NoInfer` and `@Exact` | 4 | 1 | 5 |
| H | Reified type arguments inferred to `Nothing` or a captured type | 5 | 0 | 5 |
| I | Constraints from platform types | 6 | 1 | 7 |
| J | Builder inference (PCLA) and constraint-system forks | 9 | 12 | 21 |
| K | SAM and suspend conversions during inference | 1 | 4 | 5 |
| | **Inference** | **37** | **62** | **99** |
| L | Overload resolution that depends on inference (adjacent) | 3 | 4 | 7 |

The totals above count J's two fork tests outside `pcla/`
(`forks/nestedCallVariableFixationSimple.kt`, `problems/kt30955.kt`),
and J as 19 PCLA tests plus those 2.

### A. `@OnlyInputTypes`

`kc` checks the annotation in `system.onlyInputOK`
(`internal/check/onlyinput.go`) only when no constraint is an integer
literal, a platform type, a projection or a capture, and the inferred
type has no projection. K2 checks every variable with the annotation,
after all variables are fixed, against its input types approximated as
they would be in the result, and against the components of an input
that is an intersection (`NewConstraintSystemImpl.checkOnlyInputTypesAnnotation`).

- `testsWithStdLib/inference/annotationsForResolve/kt29307.kt`: `map[42]`
  on a `Map<String, String>`. `kc` skips it for the literal.
- `testsWithStdLib/inference/annotationsForResolve/onlyInputTypes.kt`:
  `List<Inv<Number>>.contains1(Inv<Int>)` infers `Inv<out Number>`,
  whose projection `kc` skips.
- `testsWithStdLib/inference/annotationsForResolve/onlyInputTypesCaptured.kt`
- `testsWithStdLib/inference/annotationsForResolve/onlyInputTypesCommonConstraintSystem.kt`
- `testsWithStdLib/commonCollections.kt`: `ArrayList<String>.contains(1)`.
- `testsWithStdLib/java/concurrentHashMapContainsError.kt`

### B. Empty intersections

K2 fixes a variable whose upper constraints don't meet to their
intersection. If the intersection is certainly empty (two classes,
neither a subclass of the other), the candidate is inapplicable, which
resolves an overload in favour of another; with a single candidate it
is reported as `INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_ERROR`
or only a warning, depending on its kind
(`NewConstraintSystemImpl.checkInferredEmptyIntersection`). `kc` uses
`emptyIntersection` (`internal/check/subtype.go`) only while fixing a
variable with declared bounds, and only to decide whether to report a
bound violation.

- `tests/inference/intersectionTypesWithContravariantTypes.kt` and
  `tests/inference/emptyIntersectionTypes/kt51016.kt`: `kc` reports an
  ambiguity; K2 drops the candidate whose `S` would be `Long & Bar<Long>`.
- `tests/typeParameters/dontIntersectUpperBoundWithExpectedType.kt`,
  `tests/mapStringIntAsDelegate.kt`,
  `testsWithStdLib/inference/annotationsForResolve/exactAnnotation.kt`,
  `tests/resolve/specialConstructions/exclExclAsCall.kt`: K2 infers
  `Foo? & Bar?`, `Int & String` or `A & Int` and warns; `kc` reports a
  bound violation, a mismatch or that it can't infer the variable.

### C. Common supertypes and intersection types

`kc`'s common supertype (`commonSuper` and `minimalSupers` in
`internal/check/subtype.go`) takes the first shared supertype in
breadth-first order, an intersection only of Kotlin classes, `Any` for
two builtin types (`Number` for numbers), and a type parameter's bound
in place of the parameter. Smart casts make intersections only in the
cases `paramIntersection` and `narrowedIntersection`
(`internal/check/typecheck.go`) handle. K2 computes the intersection of
the minimal common supertypes (`NewCommonSuperTypeCalculator`), keeps
type parameters, and makes every smart cast an intersection with the
declared type.

- `tests/implicitIntersection.kt`: `if (b is A && b is C) b else null`
  for `fun gav(b: B): A?`. `kc`: `inferred type is C? but A? was expected`.
- `tests/inference/conflictingSubstitutions.kt`: `R` from `11` and
  `list("72")` is `Comparable<*> & Serializable` (also needs E's
  equality constraints).
- `tests/inference/kt62609.kt`: two Java classes' common supertype is
  `X<out ...> & I<out ...>`.
- `tests/problems/kt33904.kt`: the common supertype of `MODULE_SERVICE`
  and `APP_SERVICE`, both `: SERVICE`, is `SERVICE`; `kc` gives `Any`.
- `tests/inference/publicApproximation/twoIntersections.kt`
- `tests/smartCasts/intersectionScope/intersectReturnType.kt`,
  `tests/smartCasts/intersectionScope/capturedSpecificity.kt`,
  `tests/smartCasts/explicitTypeArguments.kt`, `tests/inference/kt53494.kt`:
  smart casts to a type parameter, or from one, as an intersection.
- `testsWithStdLib/intersectRawAndMutable.kt`,
  `testsWithStdLib/java/intersectFlexibleAndMutable.kt`
- `tests/integerLiterals/literalsInInference.kt`: an integer literal
  type that must become `Long` for `B : Comparable<A>`.

### D. Captured types

`kc` has no captured type. A variable that stands for a projection is
marked (`tvar.star`, `tvar.proj`, `tvar.capt` in
`internal/check/infer.go`) and approximated when substituted
(`Subst.ApplyArg`, `applyA` in `internal/types/generic.go`). K2 captures
the projections of a receiver's or argument's type into types of their
own, subtypes and infers with them, and approximates them only where a
type escapes (`TypeApproximator`).

- `tests/capturedTypesSubtyping.kt`: `x.foo { it }` on an
  `Inv<out CharSequence>`, where `foo(x: (E) -> E)`. `kc`: `inferred
  type is CharSequence but Nothing was expected`.
- `tests/inference/capturedTypes/differentCapturedTypes.kt`:
  `select(w, myE() ?: w)` for `w: Out<*>`. `kc`: can't infer `E`.
- `tests/inference/capturedTypes/cannotCaptureInProjection.kt`,
  `tests/inference/capturedTypes/kt49101.kt`: errors `kc` misses.
- `tests/scopes/variantProjections/dataClassCopy.kt`,
  `tests/generics/projectionsScope/unsafeVarianceOnInputTypeOfFunctionalType.kt`,
  `tests/generics/projectionsScope/unsafeVarianceInAliasedFunctionalType.kt`
- `tests/j+k/withTestConnection.kt`, `tests/j+k/withTestConnection2.kt`:
  a `CompletableFuture<*>` for a Java `Future<T>`.
- `testsWithStdLib/java/assertThatOnMap.kt`,
  `testsWithStdLib/intellij/WithSpeedSearch.kt`,
  `tests/j+k/rawTypesFromCaptured.kt`

### E. Incorporation and fixation order

`kc` keeps a variable's lower and upper bounds as two lists and picks
the next variable to fix by a score (`system.complete`). K2 also keeps
equality constraints, incorporates every new constraint with the others
(`ConstraintIncorporator`), and picks the next variable by a readiness
order (`VariableReadinessCalculator`, `VariableFixationFinder`) that
looks at which constraints are proper, which variables depend on which
(`TypeVariableDependencyInformationProvider`), whether the type
parameter is reified, and whether a constraint is an equality, a
platform type, an integer literal or `Nothing`. Constraints that only
come from declared bounds don't make a variable inferable.

- `tests/inference/commonSystem/postponedCompletionWithExactAnnotation.kt`
  and `..._ni.kt`: `elvisSimple(elvisSimple(nullableSample,
  materialize()), any)`. `kc`: can't infer `materialize`'s `T`.
- `tests/inference/completion/postponedArgumentsAnalysis/wrongVariableFixationOrder2.kt`
- `tests/typeParameters/dontLoseUpperNonExpectedTypeConstraints.kt`:
  `kc` infers `ModOp<M, M>` for an expected `ModOp<M, Z>`.
- `tests/delegatedProperty/provideDelegate/commonSupertype.kt`,
  `testsWithStdLib/inference/delegates/kt32249.kt`
- `tests/typealias/typeAliasConstructorTypeArgumentsInferenceWithPhantomTypes.kt`,
  `tests/inference/regressions/kt47941.kt`,
  `tests/inference/genericAssignmentOperator.kt`
- `tests/generics/kt30590.kt`: `emptyStrangeMap()` for `<T : A, R : T>`
  has only declared bounds, which K2 reports
  (`CANNOT_INFER_PARAMETER_TYPE`) and `kc` infers as `A`.

### F. Postponed arguments

`kc` checks a call's lambdas in argument order, moving those whose
parameter types are known to the front (`readyFirst`), and fixes the
variables of a lambda's parameter types when it reaches the lambda
(`runLambdas`, `fixRelated`, `postponedLambda` in
`internal/check/lambda.go`). An `if`, `when` or elvis is checked branch
by branch (`completeBranches` in `internal/check/expr.go`). K2 completes
a call in stages (`ConstraintSystemCompleter.runCompletion`): it
analyzes lambdas whose parameter types are fixed, then collects the
parameter types of the others from the whole system
(`PostponedArgumentInputTypesResolver`), fixes those variables, revises
expected types, and only then fixes the rest. A lambda whose
parameters all have types adds its function type as a constraint
without fixing anything. `if`, `when` and elvis are synthetic calls of
a generic function, so their branches' lambdas are postponed arguments
too.

- `tests/inference/completion/postponedArgumentsAnalysis/lackOfDeepIncorporation.kt`:
  `bar { c -> { b -> { a -> function(a, b, c) } } }`. `kc`: `cannot
  infer a type for this parameter`.
- `testsWithStdLib/inference/completion/postponedArgumentsAnalysis/deepLambdas.kt`:
  `listOf({ x: Color -> x.grayValue() }, Color.BLUE)`. `kc`: `inferred
  type is (Color) -> Unit but Color was expected`.
- `tests/inference/constraints/complexDependencyWihtoutProperConstraints.kt`:
  `if (bool) { id { "2" } } else null`. `kc`: `inferred type is () ->
  String but Nothing? was expected`.
- `tests/inference/kt36819.kt`, `tests/inference/extensionLambdasAndArrow.kt`:
  lambdas in `when` or `if` branches that take their parameter types
  from another branch or the expected type.
- `tests/inference/nestedCalls/secondLambdaWithRevisableExpectedType.kt`,
  `tests/inference/operators/elvisWithExpectedTypeComplex.kt`,
  `tests/inference/kt68570.kt`, `tests/resolve/starProjectionAndFunctionType.kt`,
  `tests/smartCasts/smartcastOnFunctionalType.kt`

### G. `@NoInfer` and `@Exact`

`kc` reads `@NoInfer` from library metadata for context parameters
(`types.TypeParamSym.NoInfer`, `internal/kmeta/proto.go`) but not on
source declarations or types, and doesn't know `@Exact`. K2 adds no
constraint from a `@NoInfer` position, only a check, and an `@Exact`
variable takes an equality constraint.

- `tests/inference/noInferUnit.kt`, `tests/inference/noInferUpperLower.kt`,
  `testsWithStdLib/inference/annotationsForResolve/noInferAndLowPriority.kt`,
  `testsWithStdLib/inference/annotationsForResolve/noInferMismatchingClosestReceiver.kt`,
  `testsWithStdLib/inference/annotationsForResolve/exactAnnotationWithUpperBoundConstraint.kt`

### H. Reified type arguments

K2 reports `REIFIED_TYPE_FORBIDDEN_SUBSTITUTION` when a reified type
parameter is inferred to `Nothing`, `Nothing?` or a captured type;
`kc` checks only intersections (`system.fix`).

- `tests/inference/constraints/ignoreConstraintFromImplicitInNothing.kt`,
  `tests/inference/nothingType/discriminateNothingForReifiedParameter.kt`,
  `testsWithStdLib/reified/reifiedNothingSubstitution.kt`,
  `testsWithStdLib/ArrayOfNothing.kt` (which also needs
  `UNSUPPORTED` for `Array<Nothing>`), `tests/generics/tpAsReified/CapturedAsReified.kt`

### I. Constraints from platform types

A Java method's `T` whose argument is nullable becomes nullable in K2,
so `JavaClass.transform(s)` for a `String?` is a `String?`, not a
`String!`. K2 prefers a variable's flexible lower constraint
(`HAS_PROPER_FLEXIBLE_LOWER_CONSTRAINT`) and makes the result flexible
only if a constraint is (`ResultTypeResolver.makeFlexibleIfNecessary`).
`kc` keeps the platform type (the `pa.P` cases of `system.sub`).

- `tests/inference/flexibleConstraints.kt`, `tests/inference/flexibleConstraints2.kt`,
  `tests/nullableTypes/inferenceFlexibleTToNullable.kt`,
  `tests/platformTypes/methodCall/singleton.kt`,
  `tests/j+k/genericFunAndClassNullabilityInconsistency.kt`,
  `tests/j+k/nullForOptionalOf.kt`, `tests/platformTypes/methodCall/multipleExactBoundsNullable.kt`

### J. Builder inference and forks

`kc`'s builder inference (`stubInference` in `internal/check/lambda.go`)
checks a lambda once with the builder's variables unfixed, records the
constraints that subtyping puts on them (`stubRun`), solves those, and
checks the lambda again. A builder nested in another gets a run of its
own, and a call that combines both builders' variables can't be
inferred. There are no forks. K2's PCLA (`FirPCLAInferenceSession`;
`docs/fir/pcla.md` in JetBrains/kotlin) analyzes the lambda once, with
the outer call's variables in one shared system, postpones the inner
calls that mention them, fixes a variable on demand when a member of a
value of its type is needed, and resolves the system's fork points
(`NewConstraintSystemImpl.resolveForkPointsConstraints`) only at the
end.

J1, nested builders and stubs' common supertypes (6):
`tests/inference/pcla/stubTypes/{commonSuperType,commonSuperTypeContravariant,commonSuperTypeCovariant,commonSuperTypeNullable,intersect,simpleIntersection}.kt`.
Reduced: `build { emit("1"); build2 { emit(1); select(this@build.get(), get()); "" } }`
fails with `cannot infer type for type parameter 'R'`; without the
`select` it compiles.

J2, forks (5): `tests/inference/pcla/csForks/{postponingForksNecessary,postponingForksNecessaryWithLambda,relatedToOuterGreen}.kt`,
`tests/inference/forks/nestedCallVariableFixationSimple.kt` and
`tests/problems/kt30955.kt`. A smart cast makes a receiver `A<T> & B`
(`B : A<Int>`), so `A<T> & A<Int> <: A<F>` has two solutions, `F = T`
and `F = Int`, which K2 keeps until other constraints choose.

J3, errors `kc` misses or reports wrongly in builders (10):
`tests/inference/pcla/issues/{kt47989a,kt47989b,kt52838b,kt52838c,kt74819f,kt67993}.kt`,
`tests/inference/pcla/overloadResolutionByLambdaReturnType.kt`,
`tests/inference/pcla/stubTypes/capturedTypes.kt`,
`testsWithStdLib/pcla/buildListRemoveAddInBranches.kt`,
`tests/inference/nothingType/returnAsLastStatementInLambda.kt`.

### K. Conversions during inference

`tests/suspendConversion/basicSuspendConversionGenerics.kt`,
`tests/suspendConversion/inGenericArgument.kt`, `tests/j+k/sam/kt37920.kt`,
`tests/samConversions/samConversionWithConditionJava.kt`,
`tests/samConstructors/genericSamInferenceFromExpectType.kt`: a SAM or
suspend conversion of an argument whose expected type has variables.

### L. Overload resolution (adjacent)

`tests/problems/kt24230.kt`, `tests/resolve/overloadConflicts/kt37692.kt`,
`tests/callableReference/resolve/resolveReferenceAgainstKFunctionAndKPrpoerty.kt`,
`testsWithStdLib/ambiguityWithUnsignedSpecificity.kt`,
`tests/callableReference/function/ambiguityTopLevelVsTopLevelGeneric.kt`,
`tests/j+k/sam/referenceToSamFunctionAgainstExpectedType.kt`,
`tests/j+k/selectMoreSpecific.kt`. These need resolution of callable
references and most-specific selection more than inference; B's typed
errors and E's fixation help some of them, but they aren't counted
below.

### Errors on unmarked lines

The 692 inference errors on lines without expected diagnostics, by
where they are:

| Tests | In tests kotlinc rejects | In tests kotlinc compiles |
|:---|---:|---:|
| `pcla/` (builder inference) | 148 | 94 |
| other `inference/` | 44 | 38 |
| smart casts | 28 | 33 |
| Java and platform types | 22 | 49 |
| integer literals and numbers | 22 | 1 |
| others | 151 | 62 |

The ratchet doesn't count those in tests both compilers reject, but
they are false errors, and a real program hits them the same way.
`tests/inference/pcla/stubTypes/commonSuperTypeInvariant.kt` alone has
30, and `tests/inference/pcla/fixationOnDemand/` has about 40.

## 2. How kc infers types

All of it is in `internal/check`. The type representation is in
`internal/types`.

**The system.** `system` and `tvar` (`internal/check/infer.go`). Each
type parameter of a callee becomes a variable (`system.fresh`, which
also turns a declared bound that mentions another variable into an
upper bound). An argument adds `arg <: param` with `system.sub`, which
reduces it to bounds on variables: a variable on the right gets a lower
bound (`addLower`), on the left an upper bound (`addUpper`), and a class
type is matched through its supertype's arguments (`subArgs`). A new
bound is checked against the variable's other bounds, but bounds are
not otherwise incorporated: there are no equality constraints, so an
invariant argument gives `T <: R` and `R <: T` as two separate bounds.
Integer literals are kept apart (`tvar.lits`), as are the marks for
captured projections (`star`, `proj`, `capt`). A failed constraint sets
`system.ok` to false; errors are strings (`system.errs`), reported by
`system.finish`.

**Nested calls.** A generic call that is an argument of another isn't
completed: its system merges into the outer one (`system.merge`, the
`sink` parameter of the call checker in `internal/check/call.go`), and
fixups write the final types into the tree when the outer call
finishes. Branches of an `if` or `try` that is a delegate end in the
delegate's system (`sinkBranches` in `internal/check/postponed.go`).

**Completion.** `system.complete` repeatedly picks the unfixed variable
with the best score: all bounds proper beats a proper lower bound,
which beats a proper upper bound; variables without bounds go last.
`system.fix` then chooses a value: the least upper bound of the proper
lower bounds (`Checker.lub`, joined with literal types and checked
against the upper bounds by `withinUppers`); failing that, the literal
type (`litType`); failing that, the most specific upper bound, or the
intersection of upper bounds none of which is below another; failing
that, a self-type bound (`selfTypeBounds`), the declared bound of a
captured star, a builder's hint (`stubHint`) or an error. It then
reports reified intersections, `@OnlyInputTypes` (`onlyInputOK`) and
bound violations, and substitutes the value into the other bounds.

**Common supertypes and intersections.** `Checker.join` and
`Checker.commonSuper` (`internal/check/subtype.go`): see group C.
Intersection types are synthetic type parameters bounded by their
components (`Checker.intersect`, `internIntersection` in
`internal/check/intersect.go`), made by `fix` for upper bounds, by
`minimalSupers` for Kotlin classes' shared supertypes, by `joinArgs` for
arguments, and by smart casts (`paramIntersection`,
`narrowedIntersection` in `internal/check/typecheck.go`). Declarations
visible outside approximate them by their common supertype
(`approxIntersections`). `emptyIntersection` decides whether two
classes can't meet.

**Lambdas.** A lambda argument becomes a `pendingLambda`
(`internal/check/lambda.go`). A call that is an argument of another
runs only the lambdas whose parameter types are known (`runLambdas`
with `readyOnly`) and leaves the rest to the outer call. Otherwise
`runLambdas` orders them (`readyFirst`), gives a lambda passed for a
variable a function type from the variables above it (`funcUpper`,
`openResult` in `internal/check/postponed.go`) or from callable
references (`refShape`), fixes the variables related to its inputs
(`fixRelated`), and checks it (`postponedLambda`), which first takes
declared parameter types, then fixes every variable in the parameter
types (`fixVarsIn`) and lets the body's result constrain the return
type. An explicitly typed lambda can constrain before that
(`typedLambdaParams` in `internal/check/call.go`). A parameter whose
type can't be fixed is reported at `lambda.go:407`.

**Builder inference.** `stubInference` and `stubRun`
(`internal/check/lambda.go`): when a lambda's parameter or receiver
types have variables no proper bound fixes, the lambda is checked with
them unfixed as stubs. Subtyping against a stub records a constraint in
the run instead of failing (`stubConstraint`, `stubRun.add`), other
lambdas of the call that share variables join the group, generic calls
inside the lambda are inferred with the run (`joinRun`), and a member
access on a stub fixes it on demand (`fixedStub`, `stubReceiver`). The
trial's changes to the tree are undone (`trial`, `endTrial`), the run
is solved (`stubRun.solve`, `fixAll`), and the lambda is checked again
with the variables fixed.

**Overload resolution.** `Checker.applicable` (`internal/check/call.go`)
constrains a copy of each candidate's system and keeps those whose
system is consistent, whose literals fit, whose lambdas fit and whose
variables can meet their bounds (`withinBounds`). `mostSpecific` picks
among them. Inference errors from `fix` happen after a candidate is
chosen, so they never make one inapplicable.

**`@OnlyInputTypes`.** `system.onlyInputOK` (`internal/check/onlyinput.go`),
called from `fix`: see group A.

Three things about this design shape the plan:

1. Constraints are flat lists of bounds, with no kind (lower, upper,
   equality), position or origin (argument, declared bound, expected
   type). K2's readiness order, `@OnlyInputTypes`, `@NoInfer`, the
   declared-bounds rule and its error messages all need those.
2. Errors are strings reported after resolution. K2's constraint errors
   have an applicability (`INAPPLICABLE`, `RESOLVED` for warnings),
   which is how an empty intersection drops a candidate.
3. Builder inference is a separate trial over the lambda, not part of
   the call's system, so nested builders and forks have nowhere to
   live.

## 3. The gap against K2

K2's inference is in `compiler/resolution.common/src/org/jetbrains/kotlin/resolve/calls/inference/`
(the constraint system, shared with K1) and
`compiler/fir/resolve/src/org/jetbrains/kotlin/fir/resolve/inference/`
(completion and PCLA) of JetBrains/kotlin.

| K2 | What kc lacks | Groups |
|:---|:---|:---|
| `NewConstraintSystemImpl`, `MutableConstraintStorage`: constraints with a kind and position, errors with an applicability | Kinds, positions and typed errors | A, B, E, G |
| `ConstraintIncorporator`, `ConstraintInjector`: every new constraint combined with the others, equality constraints | Equality constraints; incorporation through variables | C (conflictingSubstitutions), E |
| `VariableFixationFinder`, `VariableReadinessCalculator`, `TypeVariableDependencyInformationProvider`: readiness order | A score with three levels; no reified, flexible, literal, `Nothing` or equality priorities; declared bounds count as proper | E, I |
| `ResultTypeResolver`: result type from lower or upper constraints, flexible only if a constraint is, empty intersections allowed | Mostly there (`system.fix`); flexibility and empty intersections differ | B, I |
| `NewCommonSuperTypeCalculator`: intersection of minimal common supertypes, type parameters kept, projections | First shared supertype; `Any` for builtins | C |
| Captured types and `TypeApproximator` | Captures are marks on variables, approximated eagerly | D, H, part of A and J3 |
| `ConstraintSystemCompleter.runCompletion` and `PostponedArgumentsAnalyzer`: staged completion, input types collected from the whole system | Argument order with local heuristics | F |
| Synthetic calls for `if`, `when`, elvis, `!!`, `try` | Branch-by-branch checking | F, part of C |
| `checkInferredEmptyIntersection` and the resolution-stage check of incompatible upper bounds | Only for bound violations | B |
| `checkOnlyInputTypesAnnotation` | A subset | A |
| Fork points: `ForkPointData`, `resolveForkPointsConstraints` | None | J2 |
| `FirPCLAInferenceSession`: one shared system, postponed inner calls, fixation on demand | A trial per lambda | J1, J3 |
| `@NoInfer`, `@Exact` | Only `@NoInfer` on library context parameters | G |
| Reified checks of the inferred type | Only intersections | H |

## 4. Milestones

Each milestone lands on its own, with `bench/results/diagnostics.tsv`
and `bench/results/box.tsv` showing 0 regressions (CI's Conformance
job), and updates both baselines and unsupported.md in the same change.
Sizes: S is under 200 changed lines, M 200 to 800, L 800 to 2,000, XL
more. The expected flips are estimates from reading the tests, except
where a measurement is given.

### M0. Tooling (S)

- A harness flag, `-errors FILE`, that writes each test's `kc` errors
  on lines without expected diagnostics as TSV (path, category,
  file:line, message). This plan used a temporary version of it (in
  `bench/conformance/diagnostics.go`, where the report counts them).
  Each milestone should then show the stray inference errors going
  down, not only tests flipping.
- A test list, `bench/results/inference.txt`, of this plan's 99 tests
  and their groups, so a milestone's run can be filtered to its group.
  The harness's `-filter` takes a prefix; a `-list FILE` flag that
  takes paths would do.

Flips nothing. Risk: none.

### M1. `@OnlyInputTypes` (S)

Changes to `system.onlyInputOK` (`internal/check/onlyinput.go`):
compare an inferred type that has projections; count an integer
literal's possible types (`Int`, `Long`, `Short`, `Byte` that it fits)
as inputs; and accept an inferred type equal to a component of an
intersection input. Keep skipping inputs with captured types (star
projections from a capture, `tvar.proj`) until M10.

Measured on a prototype that dropped the literal and projection checks
(not committed): 4 tests flip (`kt29307.kt`, `onlyInputTypes.kt`,
`onlyInputTypesCaptured.kt`, `concurrentHashMapContainsError.kt`), with
1 regression,
`testsWithStdLib/inference/annotationsForResolve/moreOnlyInputTypes.kt`,
whose input is a `Class<*>` from `this`: a captured type, which the
rule above keeps skipping. `commonCollections.kt` and
`onlyInputTypesCommonConstraintSystem.kt` need the intersection rule
and M5's common supertype. The box suite was unchanged.

Expected: 4 to 6 accepts invalid fixed. Risk: false errors on calls
with captured inputs; the prototype's regression shows where.

### M2. Reified `Nothing`, `@NoInfer` and `@Exact` (S to M)

- Report `REIFIED_TYPE_FORBIDDEN_SUBSTITUTION` when a reified type
  parameter's argument, inferred or written, is `Nothing` or
  `Nothing?`. `Checker.reifiedTypes` (`internal/check/inlinefun.go`)
  already checks the final arguments of a call with reified type
  parameters, but only for non-reified type parameters. Group H,
  except `CapturedAsReified.kt` (M10) and the `UNSUPPORTED` part of
  `ArrayOfNothing.kt`. K2 doesn't report it where `Nothing` comes only
  from an implicit constraint it ignores
  (`ignoreConstraintFromImplicitInNothing.kt` has both cases), which
  needs M7's constraint origins to get exactly right.
- Read `@kotlin.internal.NoInfer` and `@kotlin.internal.Exact` on
  source type references (the tests suppress `INVISIBLE_REFERENCE` to
  use them) and from library metadata, and mark the parameter types.
  A `@NoInfer` position adds no constraint: its argument is checked
  against the type the other constraints infer. An `@Exact` return type
  adds an equality where `sub` would add a bound (until M6 has
  equalities: both bounds).

Expected: 3 to 4 of H, 4 to 5 of G. Files: `internal/check/infer.go`,
`internal/check/call.go`, `internal/check/annotations.go`,
`internal/types/types.go`. Risk: the standard library uses `@NoInfer`
and `@Exact` in signatures `kc` already compiles (`assertEquals`,
`elvis`-like helpers in kotlin-test); the box suite covers those.

### M3. Typed inference errors and empty intersections (M)

Give `inferError` a kind: error, warning, or inapplicable. An
inapplicable error found while constraining a candidate's copy in
`Checker.applicable` drops the candidate; one found after the choice is
reported as an error.

Then, as K2: when the upper constraints of a variable give an
intersection, ask `emptyIntersection` whether it is certainly empty
(two classes) or only possibly (a final class and an interface it
doesn't implement). In `applicable`, a certainly empty one makes the
candidate inapplicable if there are others. Otherwise infer the
intersection and warn
(`INFERRED_TYPE_VARIABLE_INTO_EMPTY_INTERSECTION_WARNING` and
`..._POSSIBLE_EMPTY_INTERSECTION`), instead of the bound violation `fix`
reports today when the variable has declared bounds.

This needs the upper constraints of a candidate's variables while it is
constrained, before fixing, so `withinBounds` grows into the check:
run the empty-intersection test on each variable's proper upper bounds
and declared bounds.

Expected: group B (6), and it is the base for E's error kinds. Files:
`internal/check/infer.go`, `internal/check/call.go`,
`internal/check/subtype.go`. Risk: candidates dropped that `kc`
compiles today; the box suite and the 3,935 accepts-valid tests guard
it. Performance: one `emptyIntersection` per candidate variable with
two or more upper bounds; cache by pair as K2 does
(`intersectionTypesCache`).

### M4. Constraints from platform types (S to M)

In `system.sub`, record whether a lower bound is flexible rather than
substituting the platform type, and in `fix` make the result flexible
only if all proper lower bounds are flexible; a nullable lower bound
makes it nullable (`ResultTypeResolver.makeFlexibleIfNecessary`). Put
a variable with a flexible lower bound before those whose bounds
depend on it (until M6, a tie-break in `complete`). Respect
`@NotNull`-annotated Java type parameters (`nullForOptionalOf.kt`).

Expected: group I (7). Files: `internal/check/infer.go`, Java signature
reading in `internal/check/java.go`. Risk: high in the box suite, which
calls Java generics everywhere: a value inferred as `String?` instead of
`String!` makes a later member call an error. The `pa.P` comments in
`sub` list the cases that must keep a platform type
(`SortedComboBoxModel`, `prefs.getString(k, null)`). Land behind both
baselines and a kotlinpoet compile (`bench/kotlinpoet.sh`).

### M5. K2's common supertype (M)

Rewrite `commonSuper` and `minimalSupers` (`internal/check/subtype.go`)
after `NewCommonSuperTypeCalculator`: the intersection of the minimal
common supertypes of all classes, Java interfaces and builtins
included (`Int` and `String` give `Comparable<*> & Serializable`), type
parameters kept (the supertypes of `T` include its bounds, and two
type parameters with a common bound give that bound), and arguments
joined as projections (`Inv<out Number>` for `Inv<Int>` and
`Inv<Double>`). Integer literal types join as K2's: `Long` and a
literal give `Long` where the literal fits. Keep `approxIntersections`
for what declarations expose.

Expected: `kt62609.kt`, `kt33904.kt`, `twoIntersections.kt`,
`literalsInInference.kt`, part of `conflictingSubstitutions.kt` (with
M6), `onlyInputTypesCommonConstraintSystem.kt` and `commonCollections.kt`
(with M1): about 6, and fewer stray mismatches. Risk: codegen erases
an intersection to its first component's erasure; a different first
component changes descriptors and `CHECKCAST`s, which the box suite
catches. unsupported.md documents the current `Any` result; existing
tests in `testdata/errors` rely on it. Performance: supertype closure
per pair, cached per `Checker`.

### M6. Smart casts as intersections (M)

Make every smart cast to a type that isn't a supertype of the declared
type an intersection of both, generalizing `paramIntersection` and
`narrowedIntersection` (`internal/check/typecheck.go`): classes with
interfaces, several interfaces (`b is A && b is C` for `b: B`), type
parameters with class types and other parameters, generic types
(`Inv<T> & Inv<A>` after an unchecked cast), raw and flexible types.
Member lookup on an intersection already goes through its bounds;
choose the most specific of members found in several components, as
K2's intersection scope does (`smartCasts/intersectionScope/*`).

Expected: `implicitIntersection.kt`, `intersectReturnType.kt`,
`capturedSpecificity.kt`, `explicitTypeArguments.kt`, `kt53494.kt`,
`intersectRawAndMutable.kt`, `intersectFlexibleAndMutable.kt`: about 7.
It is also the input J2's forks need. Risk: codegen of values whose
type is an intersection (erasure, casts at use); the box suite. The
smart-cast tests in the diagnostics suite (61 stray errors) guard
false errors.

### M7. Constraint storage, incorporation and readiness (L)

The core change. Replace `tvar.lower` and `tvar.upper` with
constraints that have a kind (lower, upper, equality) and an origin
(argument, receiver, expected type, declared bound, literal, `@Exact`,
fixed variable). Incorporate as `ConstraintIncorporator` does: a new
`T <: X` combines with every `Y <: T`, and an equality with both sides;
an invariant argument gives an equality, not two bounds. Replace
`system.complete`'s score with K2's readiness order
(`VariableReadinessCalculator.TypeVariableFixationReadinessQuality`):
has proper constraints (declared bounds don't count), no dependency on
outer variables, not a postponed argument's input whose variables are
unfixed, reified, flexible lower constraint, a constraint other than an
integer literal, no explicit `Nothing` lower bound, a proper equality,
a lower over an upper; ties broken by the order of variables in the
call (`ConstraintSystemCompleter.getOrderedAllTypeVariables`). A
variable with only declared-bound constraints is not inferred
(`kt30590.kt`).

To land this with no regressions, build the new order beside the old
one first: an environment variable makes `complete` compute both and
log where they choose differently, and the change switches over once
the suites show only improvements. `bench/conformance` with `-j 1` and
the log gives the list of disagreements per test.

Expected: group E (10), `conflictingSubstitutions.kt` with M5, and
the base for F. Files: `internal/check/infer.go` (most of it),
`internal/check/postponed.go`, `internal/check/lambda.go` (which read
`tvar` bounds directly), `internal/check/onlyinput.go`. Risk: the
highest of the plan in both suites, since every generic call goes
through it. Performance: incorporation is quadratic in a variable's
constraints; K2 bounds it by deduplicating constraints by type and
kind, and `kc` must too. Time `bench/kotlinpoet-time.sh` and
`bench/coroutines-time.sh` before and after; the diagnostics suite's
wall time (about 35 s here) is a quick proxy.

### M8. Postponed arguments as in K2 (L)

With M7's readiness order, restructure `runLambdas` after
`ConstraintSystemCompleter.runCompletion`:

1. Analyze lambdas whose parameter types are proper.
2. For the others, build their expected function types from all of the
   system's constraints on their variable
   (`PostponedArgumentInputTypesResolver`): a lambda passed for `T`
   whose `T` is below `(String) -> Unit` through another variable takes
   it, as `funcUpper` does now, but also from lower constraints and
   other lambdas.
3. Fix the variables of those input types, in readiness order.
4. Analyze the next lambda; repeat; fix the remaining variables.

A lambda whose parameters all have declared types, or a callable
reference with one candidate, adds its own function type as a lower
constraint and fixes nothing (`deepLambdas.kt`). Treat `if`, `when`,
elvis and `!!` as calls of a synthetic `fun <K> select(vararg k: K): K`,
so their branches' lambdas, generic calls and `null` join one system
(`complexDependencyWihtoutProperConstraints.kt`, `kt36819.kt`,
`extensionLambdasAndArrow.kt`), replacing the branch sinks of
`completeBranches` and `sinkBranches`.

Expected: group F (10) and some of K. Files: `internal/check/lambda.go`
(`runLambdas`, `readyFirst`, `fixRelated`, `ownLambda`, `refShape`),
`internal/check/postponed.go`, `internal/check/expr.go`
(`completeBranches`). Risk: lambdas checked in a different order see
different smart casts and captured variables; the box suite's
coroutine and inline tests cover it. Performance: lambdas are checked
once, as now; the synthetic calls add a system per `if`/`when` with a
generic branch, not per `if`.

### M9. Forks (M)

When `sub` meets an intersection on the left of a class type
(`A<Int> & A<T> <: A<F>`) and more than one component matches, record
a fork point with the constraints of each component instead of taking
the first. At completion, before fixing (and in builder inference, at
the end), choose for each fork point the first branch consistent with
the other constraints (`applyTheBestBranchFromForkPoint`). Needs M6 for
the intersections and M7 for constraint sets.

Expected: `nestedCallVariableFixationSimple.kt`, `kt30955.kt`, and
with M11 the three `pcla/csForks/` tests. Risk: low outside smart-cast
intersections, which are where forks come from. Performance: forks
multiply; K2 resolves each fork point independently, and so should
`kc`, with no search over combinations.

### M10. Captured types (XL)

Add a captured type to `internal/types` (the projection it captures,
its bounds, an identity per capture site), capture the projections of
a receiver's and an argument's type at each call, subtype and infer
with it, and approximate at the boundaries K2 does (`TypeApproximator`:
declarations' types, a lambda's parameter types, error messages). This
replaces `tvar.star`, `proj`, `capt` and `captBound` and much of
`Subst.ApplyArg`'s approximation.

Expected: group D (12), `CapturedAsReified.kt`, the captured parts of
A (`moreOnlyInputTypes.kt` stays green with M1's rule removed) and
`pcla/stubTypes/capturedTypes.kt`: about 14. Risk: very high; captured
types reach codegen through erasure and through every place that
prints or compares types. Interning (`types.MakeArgs`) must not merge
two captures of the same projection
(`differentCapturedTypes.kt` depends on it). Do it after M7, so the
constraint storage it plugs into is settled.

### M11. Builder inference as PCLA (XL)

Replace `stubInference`'s trial with K2's model
(`FirPCLAInferenceSession`): the builder call's variables join the
system of the statements in its lambda, which is analyzed once; a call
in the lambda that mentions them is completed in a mode that leaves
them unfixed and adds its constraints to the shared system
(`ConstraintSystemCompletionMode.PCLA_POSTPONED_CALL`); a nested builder
adds its variables to the same system, so
`select(this@build.get(), get())` joins two builders' variables; a
member access on a value of a not-yet-fixed variable's type fixes that
variable on demand from the constraints so far; fork points are
resolved when the outer call completes. Report what K2 reports when a
variable can't be inferred or a lambda's result must not fix it: an
anonymous object's member returning a stub (`kt47989a.kt`),
`removeLast()` before `add(42)` (`buildListRemoveAddInBranches.kt`).

Expected: group J1 and J3 (16), J2's `csForks` with M9 (3): about 19,
and most of the 242 stray errors in `pcla/` tests. Files:
`internal/check/lambda.go` (the stub code, about 800 lines),
`internal/check/call.go`, `internal/check/subtype.go` (`stubJoin`,
`stubConstraint`). Risk: very high for the box suite, which builds
with `buildList`, `sequence`, `flow` and `buildMap` everywhere; land it
only once M7 and M8 are in, since PCLA is a completion mode of the same
completer. The two `testdata/errors/stdlibBuilder*.kt` files cover
`kc`'s current behaviour and must keep passing.

### M12. Conversions during inference (M)

Run SAM and suspend conversions of an argument as K2 does, as part of
the argument's constraint: a function-typed value passed for a
`suspend (T) -> R` or a Java SAM whose type has variables adds the
converted type's constraint instead of failing (`samConverted`,
`suspendConverted` in `internal/check/call.go` run after inference
today). Group K (5). Independent of the others; after M7 it is simpler.

### Order and totals

| Milestone | Size | Expected flips | Depends on |
|:---|:---|---:|:---|
| M0 tooling | S | 0 | |
| M1 `@OnlyInputTypes` | S | 4 to 6 | |
| M2 reified `Nothing`, `@NoInfer`, `@Exact` | S to M | 7 to 9 | |
| M3 typed errors, empty intersections | M | 6 | |
| M4 platform-type constraints | S to M | 7 | |
| M5 common supertype | M | about 6 | |
| M6 smart-cast intersections | M | about 7 | |
| M7 constraints and readiness | L | about 11 | M3 |
| M8 postponed arguments | L | about 10 | M7 |
| M9 forks | M | 2 (5 with M11) | M6, M7 |
| M10 captured types | XL | about 14 | M7 |
| M11 PCLA | XL | about 19 | M7, M8, M9 |
| M12 conversions | M | 5 | |

M1 to M6 are independent and cheap: about 39 tests for S and M work.
M7 is the hinge; M8, M10 and M11 build on it. The sum is about 100,
the 99 inference tests and a few of L; some will fail for a second
reason once the first is gone, which a group's run after each
milestone shows.

## 5. Validation

**Running the suites.** `bench/kotlin-tests.sh` needs `KOTLIN_HOME`
with `build.txt` and `lib/kotlin-stdlib.jar`, `lib/kotlin-test.jar` and
`lib/kotlin-reflect.jar` (without versions in the names; with them, kc
compiles without the standard library and 3,619 tests change). Without
`kotlinc` installed, the three 2.4.21 jars from Maven Central and a
`build.txt` with `2.4.21` are enough:

```sh
TMPDIR=$SCRATCH KOTLIN_HOME=$SCRATCH/kh JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64 \
  sh bench/kotlin-tests.sh diagnostics -baseline bench/results/diagnostics.tsv -o "" -tsv $SCRATCH/run.tsv
TMPDIR=$SCRATCH KOTLIN_HOME=$SCRATCH/kh JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64 \
  sh bench/kotlin-tests.sh box -baseline bench/results/box.tsv -o "" -tsv ""
```

On 4 CPUs the diagnostics suite takes about 33 s and the box suite
about 16 s. During a milestone, filter to its tests
(`-filter tests/inference/`, or M0's list) and run both full suites
before each commit; CI's Conformance job fails on any regression.
Once a milestone lands, rerun both without `-o ""` and `-tsv ""` to
update the baselines and the reports.

**Unit tests.** Add a file per milestone to `testdata/errors`, which
`TestErrors` in `internal/driver/driver_test.go` runs (lines with
`// ERROR <message>`; a `stdlib` prefix adds the standard library):
`onlyInputTypesLiterals.kt` for M1, `emptyIntersection.kt` for M3,
`commonSupertype.kt` for M5, `smartCastIntersection.kt` for M6, and so
on, with a reduced case of each flipped test and of each regression the
suites found, valid and invalid lines both. Existing files that cover
inference and must keep passing: `inferenceBounds.kt`, `lambdas.kt`,
`onlyInputTypes.kt`, `refInference.kt`, `capturedTypes.kt`,
`stdlibBuilderFixationOnDemand.kt`, `stdlibBuilderStubReceiver.kt`.
For M7, a Go test in `internal/check` that builds systems directly and
checks the fixation order and values would pin the readiness rules
down without a parser in the way.

**Runtime.** The box suite (6,543 tests) runs what `kc` compiles.
`testdata/run` (`TestRun` in `internal/driver`) runs `kc`'s own
programs. Both matter most for M4, M5, M6, M10 and M11, which change
inferred types that codegen erases and casts.

**Performance.** Inference is on every generic call. For M3, M5, M7 and
M9 to M11, time `bench/kotlinpoet-time.sh`, `bench/coroutines-time.sh`
and `bench/ktor-time.sh` before and after (docs/benchmark.md), and keep
an eye on the suites' wall time. A regression over 3% in `kc -j 1` on
kotlinpoet needs a reason in the change.

## 6. unsupported.md

| Milestone | Bullet | Change |
|:---|:---|:---|
| M1 | Differences > Checking: "`@OnlyInputTypes` ... is checked only when ... no platform types, projections or captured types, and aren't literals." | Rewrite: checked except where an input is a captured type. |
| M10 | The same bullet | Delete. |
| M3, M5, M6 | Differences > Checking: "Intersection types (`A & B`) are inferred only for ..." | Rewrite after each: M5 removes "The common supertype of two builtin types is `Any` ..."; M6 removes the list of smart casts; M3 adds nothing to it but lets empty intersections be inferred with a warning. Delete once M6 lands, if the primitive and value-class smart cast case is done too. |
| M8 | Differences > Checking: "A lambda's parameter types are fixed from the constraints known when the lambda is reached ..." | Delete. |
| M11 | The language: "Builder inference of a builder nested in another ... and where Kotlin forks the constraint system ..." | Delete (forks need M9). |
| M2, M4, M7, M12 | None | Nothing listed; these are differences unsupported.md doesn't mention, which this plan found. |
