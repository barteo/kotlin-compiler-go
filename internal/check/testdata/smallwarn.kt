// Small warnings, each checked against kotlinc 2.4.

@Target(AnnotationTarget.EXPRESSION)
@Retention(AnnotationRetention.SOURCE)
annotation class Ann

@SinceKotlin("99.0") // NEWER_VERSION_IN_SINCE_KOTLIN
fun future() {}

@SinceKotlin("2.4")
fun current() {}

fun <T> T.apply2(f: T.() -> Unit) { f() }

fun receivers() {
    "a".apply2 outer@{
        1.apply2 outer@{
            println(this@outer) // LABEL_NAME_CLASH
        }
    }
    "a".apply2 a@{
        1.apply2 b@{
            println(this@a.length)
        }
    }
}

fun annotated(x: Int): Int {
    var y = x
    @Ann x * x // ANNOTATIONS_ON_BLOCK_LEVEL_EXPRESSION_ON_THE_SAME_LINE
    @Ann() (x * x)
    @Ann
    x * x
    y += x
    @Ann x shl 1 // ANNOTATIONS_ON_BLOCK_LEVEL_EXPRESSION_ON_THE_SAME_LINE
    return y
}

enum class E { A, B }

fun narrowedNull(e: E, something: Any?): Int {
    if (something != null) return 0
    return when (e) {
        E.A -> 1
        E.B -> 2
        something -> 3 // SENSELESS_NULL_IN_WHEN
    }
}

fun nullTwice(arg: Any?): Int {
    when (arg) {
        null -> return -1
        is String -> return 0
        null -> return 3 // SENSELESS_NULL_IN_WHEN DUPLICATE_BRANCH_CONDITION_IN_WHEN
        else -> return 5
    }
}

fun nullOnce(arg: String?): Int = when (arg) {
    "a" -> 1
    null -> 2
    else -> 3
}
