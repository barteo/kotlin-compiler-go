// DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES: one member
// inherited from several supertypes whose parameter names differ.

interface A { fun foo(x: Int) }
interface B { fun foo(y: Int) }
interface SameAsA { fun foo(x: Int) }

interface C : A, B // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
abstract class D : A, B // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
interface Same : A, SameAsA

open class Impl { open fun foo(q: Int) {} }
class FromClass : Impl(), A // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES

interface Sub : A
interface Through : Sub, B // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
interface Redundant : Sub, A
interface FromC : C

interface Overrides : A, B {
    override fun foo(x: Int) // PARAMETER_NAME_CHANGED_ON_OVERRIDE
}

interface G1 { fun <T> bar(a: T, b: Int) }
interface G2 { fun <T> bar(a: T, c: Int) }
interface G3 : G1, G2 // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
