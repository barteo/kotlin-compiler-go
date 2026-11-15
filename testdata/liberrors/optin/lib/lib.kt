package lib

@RequiresOptIn
@Retention(AnnotationRetention.BINARY)
@Target(
    AnnotationTarget.CLASS, AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY,
    AnnotationTarget.PROPERTY_SETTER, AnnotationTarget.CONSTRUCTOR, AnnotationTarget.TYPEALIAS,
)
annotation class Exp

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
@Retention(AnnotationRetention.BINARY)
annotation class Soft

@Exp
class ExpClass {
    fun member() = 1
    class Nested
}

@Exp
fun expFun() = 1

@Soft
fun softFun() = 1

@Exp
val expVal = 1

@Exp
var expVar = 1

class Holder @Exp constructor(val x: Int) {
    @OptIn(Exp::class)
    constructor() : this(0)

    @Exp
    val prop = 1

    @property:Exp
    var both = 1

    @set:Exp
    var setterOnly = 1

}

open class Base {
    @Exp
    open fun hook() = 1
}

@Exp
typealias ExpAlias = String

@SubclassOptInRequired(Exp::class)
open class Sealedish

@Exp
fun Int.ext() = this

@OptIn(Exp::class)
fun takesExp(e: ExpClass?) = 1

data class Data(@property:Exp val x: Int, val y: Int)
