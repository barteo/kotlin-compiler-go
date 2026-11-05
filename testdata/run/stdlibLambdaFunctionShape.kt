var obj: Any? = 7

@Suppress("UNCHECKED_CAST")
fun <T> materialize(): T = obj as T

fun <K> select(a: K, b: K) = b

fun main() {
    // The lambdas are () -> R, R of run's, which the Int infers.
    val x: Int = run(select({ materialize() }, fun() = materialize()))
    println(x + 1)
}
