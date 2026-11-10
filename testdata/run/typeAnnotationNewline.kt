// A type annotation can end its line, before the type it annotates.

@Target(AnnotationTarget.TYPE)
annotation class Marked

fun twice(
    f:
        @Marked
        (Int) -> Int,
    x: Int,
): Int = f(f(x))

fun main() {
    val inc:
        @Marked
        (Int) -> Int =
        { it + 1 }
    println(twice(inc, 1))
    val n: @Marked
        Int = 3
    println(n)
}
