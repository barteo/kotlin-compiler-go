@file:JvmMultifileClass
@file:JvmName("Props")

// A multi-file class's part holds its properties' fields, which the
// facade's accessors reach through the part's: a @JvmField can't be one.

@JvmField // ERROR JvmField cannot be applied to top level property of a file annotated with JvmMultifileClass
val field = 1

val plain = 2

private val hidden = 3

const val CONST = 4

fun main() {
    println(field + plain + hidden + CONST)
}
