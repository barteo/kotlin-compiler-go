@Target(AnnotationTarget.EXPRESSION)
@Retention(AnnotationRetention.SOURCE)
annotation class Ann

fun main() {
    val f = lambda@ ({ x: Int ->
        if (x > 0) return@lambda "positive"
        "other"
    })
    val g = lambda@ @Ann { x: Int ->
        if (x > 0) return@lambda "big"
        "small"
    }
    println(f(1) + " " + f(-1))
    println(g(5) + " " + g(0))
}
