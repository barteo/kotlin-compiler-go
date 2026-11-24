open class Final {
    fun foo() {}
    val bar: Int = 0
}

open class Open {
    open fun foo() {}
}

open class Derived : Final()

interface IFoo {
    fun foo()
}

class CFoo : IFoo {
    override fun foo() {}
}

interface IBarT<T> {
    val bar: T
}

class CBarT<T> : IBarT<T> {
    override val bar: T get() = null!!
}

class Test1 : Final(), IFoo by CFoo() // ERROR overrides a final member of a superclass
class Test2 : Derived(), IFoo by CFoo() // ERROR overrides a final member of a superclass
class Test3 : Final(), IBarT<Int> by CBarT<Int>() // ERROR overrides a final member of a superclass
class Test4 : Open(), IFoo by CFoo()
class Test5 : IFoo by CFoo()
