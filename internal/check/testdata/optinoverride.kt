// The OPT_IN_OVERRIDE warnings kotlinc 2.4.21 reports of the overrides
// below, as TestWarningsOptInOverride reads them: each line ends with a
// comment naming the warnings it reports, if any.
@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
@Retention(AnnotationRetention.BINARY)
annotation class E1

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
@Retention(AnnotationRetention.BINARY)
annotation class E3

interface Base1 {
    @E1
    fun foo()
}

interface Base2 {
    fun foo()
}

interface Base3 {
    @E3
    fun foo()
}

class DerivedA : Base1, Base2, Base3 {
    override fun foo() {} // OPT_IN_OVERRIDE
}

class DerivedB : Base1, Base3 {
    @E3
    override fun foo() {} // OPT_IN_OVERRIDE
}

class DerivedC : Base1, Base2, Base3 {
    @E1
    @E3
    override fun foo() {}
}

class DerivedD : Base1 {
    @OptIn(E1::class)
    override fun foo() {}
}

@E1
class DerivedE : Base1 {
    override fun foo() {}
}

open class Base {
    @E1
    open fun bar() {}
    @E1
    open val p: Int get() = 1
}

class DerivedF : Base() {
    override fun bar() {} // OPT_IN_OVERRIDE
    override val p: Int get() = 2 // OPT_IN_OVERRIDE
}

@OptIn(E1::class)
class DerivedG : Base() {
    override fun bar() {}
}
