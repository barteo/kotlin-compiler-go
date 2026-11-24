package base

abstract class Base {
    fun foo(): String = internalFoo()
    internal abstract fun internalFoo(): String
    internal abstract val internalProp: Int
}

open class BaseWithOverride : Base() {
    override fun internalFoo(): String = ""
    override val internalProp: Int get() = 1
}

abstract class Concrete {
    internal open fun hook() {}
}

abstract class Intermediate : Base()
