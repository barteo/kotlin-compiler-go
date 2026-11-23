// The PARAMETER_NAME_CHANGED_ON_OVERRIDE warnings below, as
// TestWarningsParamName reads them: each line ends with a comment naming
// the warnings it reports, if any. They follow kotlinc 2.4.21's
// diagnostics tests changeOnOverrideDiagnostic.kt,
// namedArgumentsInOverrides.kt and invokeInFunctionClass.kt.
interface A {
    fun b(a: Int)
}
interface B : A
interface C {
    fun b(c: Int)
}

class C1 : A {
    override fun b(b: Int) {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
}
class C2 : B {
    override fun b(b: Int) {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
}
class C3 : A {
    override fun b(a: Int) {}
}
class C4 : A, C {
    override fun b(a: Int) {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
}
open class D1 : A {
    override fun b(d: Int) {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
}
class D2 : D1() {
    override fun b(d: Int) {}
}
class F : (String) -> Unit {
    override fun invoke(name: String) {}
}
// The members of Kotlin's built-in classes have Kotlin's names.
class PnA { override fun equals(o: Any?) = true; override fun hashCode() = 0 } // PARAMETER_NAME_CHANGED_ON_OVERRIDE
abstract class PnL : List<String> { override fun contains(o: String) = true } // PARAMETER_NAME_CHANGED_ON_OVERRIDE
abstract class PnS : CharSequence {
    override fun subSequence(a: Int, endIndex: Int): CharSequence = "" // PARAMETER_NAME_CHANGED_ON_OVERRIDE
    override fun get(i: Int) = 'a' // PARAMETER_NAME_CHANGED_ON_OVERRIDE
}
abstract class PnC : Comparable<PnC> { override fun compareTo(o: PnC) = 0 } // PARAMETER_NAME_CHANGED_ON_OVERRIDE
abstract class PnE : MutableMap.MutableEntry<String, String> { override fun setValue(newValue: String) = "" }
abstract class PnJ : java.util.AbstractList<String>() { override fun get(i: Int) = "" }

// A Java interface's equals(Object), as Comparator's, doesn't override
// Any's; a Java class's does, as AbstractList's.
interface PnEq { override fun equals(other: Any?): Boolean }
class PnEq1 : PnEq { override fun equals(x: Any?) = true } // PARAMETER_NAME_CHANGED_ON_OVERRIDE
abstract class PnEq2 : Comparator<String> { override fun equals(x: Any?) = true } // PARAMETER_NAME_CHANGED_ON_OVERRIDE
abstract class PnEq3 : java.util.AbstractList<String>() { override fun equals(x: Any?) = true }
