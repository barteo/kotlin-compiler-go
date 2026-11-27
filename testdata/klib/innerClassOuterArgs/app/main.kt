// A type of an inner class without type parameters of its own has its
// outer class's type arguments, and its supertypes are of them:
// Call<S>.Args is an ArrayList<String?> and Call<String>.Names an
// ArrayList<String>.

import q.Call
import q.NamedCall

fun first(call: Call<*>): String? = call.arguments.getOrNull(0)

fun render(call: Call<*>): String =
    (call.arguments zip listOf(1, 2)).filter { it.second > 0 }.joinToString { "${it.first}=${it.second}" }

fun main() {
    val call = NamedCall("f")
    println(first(call))
    call.arguments.add("a")
    call.arguments.add(null)
    val all: List<String?> = call.arguments
    println("${first(call)} ${all.size} ${render(call)} ${call.arguments[1] ?: "none"}")
    call.names.add("x")
    val name: String = call.names.first()
    println("$name ${name.length} ${call.tagged(3).describe()}")
}
