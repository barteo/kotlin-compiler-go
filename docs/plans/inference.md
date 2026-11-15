# Inference: open cases

This file collects failing diagnostics tests whose cause is type
inference, or the analysis of lambdas and callable references, so a
later plan can take them as a theme. It has no milestones yet.

## From the statics and ambiguity plan

These seven fail on ambiguity diagnostics, but the ambiguity is a symptom: the
fix lies in inference. [Statics and ambiguity](statics-and-ambiguity.md)
left them here. Paths are under `compiler/testData/diagnostics/`.

**Accepts invalid**

- `tests/callResolution/kt50386.kt`: two generic `presenter` overloads
  with a lambda returning a generic class. K2 reports
  `OVERLOAD_RESOLUTION_AMBIGUITY`; `kc` picks one.
- `tests/resolve/overloadConflicts/emptyLambdaUnitNoDiscrimination.kt`:
  an empty lambda must not prefer `() -> Unit` over another function type
  (K2's `byLambdaResult` discrimination doesn't apply to it).
- `tests/unsignedTypes/unsignOverloadVsNonIntLike.kt`: after
  `a as UByte` (`CAST_NEVER_SUCCEEDS`), `a` is `String & UByte`, so both
  `foo`s apply. This is a smart cast after an impossible cast.
- `tests/secondaryConstructors/generics3.kt`: `super(x, 1)` with
  `x: T1`, where `T2 : T1` is wanted. This is subtyping of type
  parameters in delegation calls.

**Rejects valid**

- `tests/callableReference/resolve/resolveReferenceAgainstKFunctionAndKPrpoerty.kt`
  (`ambiguous call: 2 candidates for 'foo' take (lambda)`): explicit type
  arguments should leave only the `KProperty1` overload fitting.
- `tests/problems/kt24230.kt` (`ambiguous call: 2 candidates for 'bar'
  take (lambda, Int, lambda)`): a callable reference together with a
  lambda.
- `testsWithStdLib/ambiguityWithUnsignedSpecificity.kt` (`ambiguous call:
  3 candidates for 'of' take (K)`): an argument whose type variable isn't
  inferred yet, against `ULong`, `Int` and `Long`. K2 resolves it by
  specificity after inference.

## The wider picture

Of the 105 tests `kc` rejects valid (`bench/results/diagnostics.md`),
18 fail first on inference (a type variable or lambda parameter not
inferred, a bound, a recursive type) and many of the 25 type mismatches
are inferred types. `docs/unsupported.md`, under *Differences from
kotlinc*, **Checking**, lists the known gaps against K2's inference.
