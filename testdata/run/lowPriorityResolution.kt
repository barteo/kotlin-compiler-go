// @LowPriorityInOverloadResolution candidates lose to any other that fits.

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
@kotlin.internal.LowPriorityInOverloadResolution
fun foo(i: Int) = 1

fun foo(a: Any) = 2

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
@kotlin.internal.LowPriorityInOverloadResolution
fun bar(a: String?) = 3

fun bar(a: Any) = 4

fun main() {
    println(foo(1))
    println(bar(null))
    println(bar("s"))
}
