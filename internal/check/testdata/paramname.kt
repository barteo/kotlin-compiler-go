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
