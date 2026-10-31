// Code whose behavior depends on compiling as kotlinc does: a negative
// literal is one literal, which may be a Short or an Int where its
// positive value is a Long; an override's setter has bridges to the
// overridden setters; a reference converted to a fun interface equals
// another to the same function; an initializer of a field's default value
// doesn't run; a jump out of a try runs its finally block outside the
// try's handlers, also with inlined calls in it; a local class in a
// constructor or a local function has its simple name; a private member
// of another class doesn't hide an extension property; trimIndent of a
// constant is a constant; and a generic call inferred to return Nothing
// throws KotlinNothingValueException if it returns.
import kotlin.reflect.KClass

fun <T> kind(a: T, b: T) = "${(a as Any).javaClass.simpleName} ${(b as Any).javaClass.simpleName}"

interface Sized<T> {
    var size: T
}

class Box(override var size: String) : Sized<String>

open class Base<T>(init: T) {
    var size: T = init
}

interface IntSized {
    var size: Int
}

class Inherits : IntSized, Base<Int>(56)

abstract class Demo {
    internal open var value: Int = 1
}

open class Public : Demo() {
    public override var value: Int = 2
}

class Sub : Public() {
    override var value: Int = 3
}

fun interface Action {
    fun run()
}

fun id(a: Action): Any = a

fun hello() {}

fun String.shout() {}

class Order(start: Int) {
    init {
        set(start)
    }

    fun set(v: Int) {
        field = v
    }

    var field: Int = 0
    var name: String? = null
}

val sb = StringBuilder()

fun leave() {
    try {
        try {
            return
        } catch (e: Error) {
            sb.appendLine("inner catch")
        } finally {
            sb.appendLine("finally")
            throw Error()
        }
    } catch (e: Error) {
        sb.appendLine("outer catch")
    }
}

fun simpleName(k: KClass<*>) = k.simpleName

class InConstructor {
    val names: List<String?>

    init {
        class `Local$Dollar`
        names = listOf(simpleName(`Local$Dollar`::class))
    }
}

fun inLocalFun(): String? {
    fun local(): String? {
        class `Local$Dollar`
        return simpleName(`Local$Dollar`::class)
    }
    return local()
}

val Hidden.secret get() = "extension"

class Hidden {
    private val secret = "member"

    fun own() = secret
}

fun text() = """
    a
      b
""".trimIndent()

fun <T> unsafe(): T = Any() as T

fun <T> Any?.decoded(): T = this?.let { unsafe<T>() } ?: throw AssertionError()

fun main() {
    println(kind(-1, Char.MAX_VALUE.toShort()))
    println(kind(-2147483648, 1))
    println(listOf<Any>(-2147483648, -2147483649, -1L, -(1)).map { it::class.simpleName })

    val s: Sized<String> = Box("a")
    s.size = "b"
    println(s.size)
    val i: IntSized = Inherits()
    i.size = 57
    println(i.size)
    val d: Demo = Sub()
    d.value = 30
    println("${d.value} ${(d as Sub).value}")

    println(id(::hello) == id(::hello))
    println(id(::hello).hashCode() == id(::hello).hashCode())
    println("${id("a"::shout) == id("a"::shout)} ${id("a"::shout) == id("b"::shout)}")

    val o = Order(1)
    println("${o.field} ${o.name}")

    leave()
    print(sb)

    println(InConstructor().names)
    println(inLocalFun())

    println("${Hidden().secret} ${Hidden().own()}")

    println(text() === text())
    println(text())

    try {
        "x".decoded<Nothing>()
    } catch (e: RuntimeException) {
        println(e::class.simpleName)
    }
}
