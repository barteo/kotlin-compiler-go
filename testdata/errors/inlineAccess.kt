open class Base {
    open fun f() = 1
    open val p = 1
    protected fun prot() = 1
    protected val protVal = 1
    internal fun inter() = 1
}

class Derived : Base() {
    override fun f() = 2

    inline fun superCall() = super.f() // ERROR accessing super members from public-API inline function is prohibited
    inline fun superProp() = super.p // ERROR accessing super members from public-API inline function is prohibited
    inline fun inLambda() = run { super.f() } // ERROR accessing super members from public-API inline function is prohibited
    inline fun protCall() = prot() // ERROR protected function call from public-API inline function is prohibited: 'prot'
    inline fun protRead() = protVal // ERROR protected property call from public-API inline function is prohibited: 'protVal'

    private inline fun privSuper() = super.f() + prot()
    internal inline fun internalSuper() = super.f() + prot()
    fun notInline() = super.f() + prot()

    inline fun ownObject() = object : Base() {
        override fun f() = super.f() + prot() + this.prot()
    }

    inline fun outerSuper() = object {
        fun g() = super@Derived.f() // ERROR accessing super members from public-API inline function is prohibited
    }

    inline fun internalInObject() = object : Base() {
        override fun f() = inter() // ERROR public-API inline function cannot access non-public-API function 'inter'
    }
}

open class Guarded protected constructor() {
    inline fun make() = Guarded() // ERROR protected constructor call from public-API inline function is prohibited: 'Guarded'
}

private class Hidden : Base() {
    inline fun fine() = super.f() + prot()
}

fun main() {}
