@RequiresOptIn
annotation class Exp

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class Soft

@Exp
fun risky() = 1

@Soft
fun mild() = 2

@Exp
class Marked {
    fun member() = 3
    class Nested
}

fun use1() = risky() // ERROR needs opt-in
fun use2() = mild()

@OptIn(Exp::class)
fun use3() = risky()

@Exp
fun use4() = risky()

fun use5(): Int {
    @OptIn(Exp::class)
    val x = risky()
    return x
}

fun use6(m: Marked) = 1 // ERROR needs opt-in
fun use7() = Marked.Nested() // ERROR needs opt-in

@OptIn(Exp::class)
fun use8(m: Marked) = m.member()

@SubclassOptInRequired(Exp::class)
open class Base

class Sub1 : Base() // ERROR needs opt-in to inherit

@SubclassOptInRequired(Exp::class)
class Sub2 : Base()

@OptIn(Exp::class)
class Sub3 : Base()

annotation class NotAMarker

@SubclassOptInRequired(NotAMarker::class) // ERROR is not an opt-in requirement marker
open class Base2

open class Api {
    @Exp
    open fun f() {}
}

class Impl : Api() {
    override fun f() {} // ERROR this override needs opt-in
}

@RequiresOptIn
@Retention(AnnotationRetention.SOURCE) // ERROR can't have SOURCE retention
annotation class BadRetention

@RequiresOptIn
@Target(AnnotationTarget.EXPRESSION) // ERROR can't be used on target
annotation class BadTarget

fun param(@Exp x: Int) = x // ERROR can't be used on this target

@Suppress("OPT_IN_USAGE_ERROR")
fun suppressed() = risky()
