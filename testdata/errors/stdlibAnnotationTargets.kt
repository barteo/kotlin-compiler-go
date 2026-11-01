@file:JvmName("Targets")
@file:Fn // ERROR this annotation is not applicable to target 'file' and use-site target '@file'

// Where annotations are written, whether they repeat, and whether their
// arguments and default values are constants.

import kotlin.reflect.KClass

@Target(AnnotationTarget.FUNCTION) annotation class Fn
@Target(AnnotationTarget.PROPERTY_GETTER) annotation class Getter
@Target(AnnotationTarget.FIELD) annotation class OnField
@Target(AnnotationTarget.TYPEALIAS) annotation class OnAlias
@Target() annotation class Nowhere
annotation class Plain(val s: String = "")
annotation class Num(vararg val i: Int)
annotation class Cls(val k: KClass<*>)
annotation class Choice(val e: E)
annotation class Names(val names: Array<String>)
@Repeatable annotation class Again(val i: Int)

enum class E { A, B }

const val CONST = 1
const val TEXT = "text"
val notConst = 2
var mutable = 3
fun f() = 4

object Holder {
    const val INNER = 5
    val plain = 6
}

val holder = Holder

@Fn // ERROR this annotation is not applicable to target 'class'
class C1 @Fn constructor() { // ERROR this annotation is not applicable to target 'constructor'
    @Fn fun ok() {}

    @Fn // ERROR this annotation is not applicable to target 'member property'
    val p = 1

    @JvmStatic constructor(x: Int) : this() // ERROR this annotation is not applicable to target 'constructor'
}

@Nowhere fun nowhere() {} // ERROR this annotation is not applicable to target 'top level function'

fun params(@Fn x: Int) = x // ERROR this annotation is not applicable to target 'value parameter'

@Getter val g1 = 1 // ERROR this annotation is not applicable to target 'top level property'
val g2: Int
    @Getter get() = 2
val g3: Int
    @Fn get() = 3 // ERROR this annotation is not applicable to target 'getter'

@get:Getter val g4 = 4
@get:Fn val g5 = 5 // ERROR this annotation is not applicable to target 'top level property' and use-site target '@get'
@get:JvmName("g6x") val g6 = 6
@field:OnField val g7 = 7
@OnField val g8 = 8
@field:Plain fun g9() {} // ERROR this annotation is not applicable to target 'top level function' and use-site target '@field'

class Props(@OnField val a: Int, @Plain b: Int, @param:Plain val c: Int)

annotation class Ann(@OnField val x: Int) // ERROR this annotation is not applicable to target 'value parameter'

enum class Entries {
    @Plain A,
    @Fn B, // ERROR this annotation is not applicable to target 'enum entry'
}

@OnAlias @Deprecated("") typealias T1 = String
@Plain typealias T2 = String // ERROR this annotation is not applicable to target 'typealias'

@Plain @Plain class Twice // ERROR this annotation is not repeatable
@Again(1) @Again(2) class Repeated

@Plain @get:Plain val notRepeated = 1
@Plain
@property:Plain // ERROR this annotation is not repeatable
val repeated = 1
val accessor: Int
    @Plain @Plain get() = 1 // ERROR this annotation is not repeatable

@Suppress("UNUSED") @Deprecated("x") @PublishedApi internal fun suppressed() {}

// Arguments and default values must be constants.
@Num(CONST, 1 + CONST, -CONST, Holder.INNER, Int.MAX_VALUE, "abc".length, 'a'.code, CONST.toLong().toInt())
@Plain("$TEXT ${CONST + 1}" + TEXT)
@Cls(String::class)
@Choice(E.A)
@Names(["a", TEXT])
class Constants

@Names(arrayOf("a", "b" + TEXT, """
    x
""".trimIndent()))
class MoreConstants

@Num(notConst) // ERROR annotation argument must be a compile-time constant
class NotConst

@Num(CONST, mutable) // ERROR annotation argument must be a compile-time constant
class Mutable

@Num(f()) // ERROR annotation argument must be a compile-time constant
class Call

@Num(Holder.plain) // ERROR annotation argument must be a compile-time constant
class ObjectVal

@Num(holder.INNER) // ERROR annotation argument must be a compile-time constant
class ThroughVal

@Plain("a" + E.A) // ERROR annotation argument must be a compile-time constant
class EnumConcat

@Names(arrayOf(TEXT, "$notConst")) // ERROR annotation argument must be a compile-time constant
class ArrayElem

annotation class Defaults(
    val a: Int = 1 + CONST,
    val b: Array<String> = arrayOf(TEXT),
    val c: KClass<*> = Int::class,
    val d: Plain = Plain("x"),
    val e: Int = notConst, // ERROR default value of annotation parameter must be a compile-time constant
    val f: Int = f() + 1, // ERROR default value of annotation parameter must be a compile-time constant
)

@Target(AnnotationTarget.TYPE) annotation class OnType

context(@OnType s: String) // ERROR this annotation is not applicable to target 'value parameter'
val contextual: Int get() = s.length

context(@Plain s: String)
fun contextualFun() = s
