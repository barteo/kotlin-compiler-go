// References to built-in members as kotlin.reflect objects: KFunctionN
// parameters take them, and they call the member.
import kotlin.reflect.KFunction1
import kotlin.reflect.KFunction2

fun <T, R> name1(f: KFunction1<T, R>) = f.name
fun <A, B, R> name2(f: KFunction2<A, B, R>) = f.name

fun main() {
    println(name1(String::toInt))
    println(name2(Int::and))
    println(name2(String::plus))
    val f: KFunction1<String, Int> = String::toInt
    println(f("12"))
    val g: KFunction2<Int, Int, Int> = Int::and
    println(g(6, 3))
    println(g == Int::and)
}
