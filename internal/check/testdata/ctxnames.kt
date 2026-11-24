// The warnings kotlinc 2.4.21 reports of the context parameters' names below, as
// TestWarningsContextParameterNames reads them: each line ends with a comment naming
// the warnings it reports, if any.
interface I1 {
    context(x: Int) fun foo()
    context(x: Int, y: String) fun bar(z: Long)
    context(_: Int) fun baz()
    context(x: Int) val p: Int
}
interface I2 {
    context(y: Int) fun foo()
    context(x: Int, y: String) fun bar(z: Long)
    context(_: Int) fun baz()
    context(y: Int) val p: Int
}
interface I2b {
    context(_: Int) fun foo()
}
interface I3 : I1, I2 // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
interface I4 : I1, I2b { // DIFFERENT_NAMES_FOR_THE_SAME_PARAMETER_IN_SUPERTYPES
    context(_: Int) override val p: Int
}
abstract class C1 : I1 {
    context(xx: Int) override fun foo() {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
    context(x: Int, yy: String) override fun bar(zz: Long) {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
    context(named: Int) override fun baz() {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
    context(xx: Int) override val p: Int get() = 1
}
abstract class C2 : I1 {
    context(_: Int) override fun foo() {} // PARAMETER_NAME_CHANGED_ON_OVERRIDE
    context(x: Int, y: String) override fun bar(z: Long) {}
    context(_: Int) override fun baz() {}
    context(_: Int) override val p: Int get() = 1
}
abstract class C3 : I1 {
    @Suppress("PARAMETER_NAME_CHANGED_ON_OVERRIDE")
    context(xx: Int) override fun foo() {}
}
