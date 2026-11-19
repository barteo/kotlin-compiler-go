// A class can't have both a suspend and a non-suspend function type
// among its supertypes, directly or through another supertype
// (MIXING_SUSPEND_AND_NON_SUSPEND_SUPERTYPES), reported at its supertype
// list, as kotlinc does.
interface ISuper : () -> Unit
interface SISuper : suspend () -> Unit
abstract class CSuper : () -> Unit

interface I : suspend () -> Unit, ISuper // ERROR mixing suspend and non-suspend supertypes
interface I1 : SISuper, () -> Unit // ERROR mixing suspend and non-suspend supertypes
abstract class C : CSuper(), SISuper // ERROR mixing suspend and non-suspend supertypes
interface Ok : SISuper, suspend () -> Unit
interface Ok1 : ISuper, (Int) -> Unit
