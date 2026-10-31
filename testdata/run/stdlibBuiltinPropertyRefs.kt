// References to String's length, Enum's name and ordinal, and properties
// of bound primitive and value class receivers are KProperty objects.

import kotlin.reflect.KProperty0
import kotlin.reflect.KProperty1

enum class E { I, J }

@JvmInline
value class Z(val x: Int) {
    val xx get() = x + 1
}

@JvmInline
value class S(val x: String)

val Z.ext get() = x * 2

fun main() {
    val length: KProperty1<String, Int> = String::length
    println(length.get("ab"))
    println(length.name)
    val bound = "kotlin"::length
    println(bound.get())
    println((E::name).get(E.J))
    println((E::ordinal).get(E.J))
    println(E::name.name)
    val z: KProperty0<Int> = Z(42)::x
    println(z.get())
    println(Z(1)::xx.get())
    println(Z(3)::ext.get())
    println(S("s")::x.get())
    println(5::hashCode.invoke())
}
