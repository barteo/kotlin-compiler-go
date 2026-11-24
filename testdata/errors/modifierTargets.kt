interface Foo<T>
interface Foo2<in in T> // ERROR repeated 'in'
fun f1(foo: Foo<out out Int>) = foo // ERROR repeated 'out'
fun f2(foo: Foo<in out Int>) = foo // ERROR is incompatible with

abstract class Props {
    abstract var a: Int abstract get abstract set // ERROR not applicable to 'getter'
    abstract var c: Int
        open set // ERROR modifier 'open' is not applicable to 'setter'
}

enum class E {
    public A, // ERROR modifier 'public' is not applicable to 'enum entry'
    inner B, // ERROR modifier 'inner' is not applicable to 'enum entry'
    C
}

companion class NotObject // ERROR modifier 'companion' is not applicable to 'class'
const var notVal = 1 // ERROR modifier 'const' is not applicable to 'vars'

class Ctors {
    constructor(val x: Int) // ERROR 'val' on secondary constructor parameter is not allowed
}
fun withVar(var x: Int) = x // ERROR 'var' on function parameter is not allowed

val nullableSuspend: suspend (() -> Unit)? = null // ERROR modifier 'suspend' is not applicable to 'nullable type'

fun locals() {
    enum class L { X } // ERROR local enum classes are not allowed
    abstract val noType // ERROR this variable must either have an explicit type or be initialized
}
