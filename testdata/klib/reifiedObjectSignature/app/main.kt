import lib.describe
import lib.typeRef

data class Msg(val id: String)

fun main() {
    println(typeRef<Msg>().type)
    println(typeRef<List<Msg>>().type)
    println(describe<Map<String, Msg?>>())
    more()
}

inline fun <reified T> nested(): String = describe<Set<T>>()

fun more() {
    println(nested<Msg>())
    println(typeRef<IntArray>().type)
    println(typeRef<Int>().type)
}
