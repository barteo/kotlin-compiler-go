open class Dec<T>(val name: String)
class A<T> : Dec<T>("a")
class B<T> : Dec<T>("b")
fun <T> a(): A<T> = A()
fun <T> b(): B<T> = B()
interface Key
object Home : Key

fun <T : Any> show(stack: List<T>, decs: List<Dec<T>> = listOf(), p: (T) -> String) {
    println(stack.map(p) + decs.map { it.name })
}

fun main() {
    val stack = listOf<Key>(Home)
    show(stack, listOf(a(), b())) { it.javaClass.simpleName }
    show(stack = stack, decs = listOf(a(), b(), a()), p = { "k" })
}
