// A call in a builder's lambda whose variables only its own completion
// infers, once the builder's are: flatMap's R from the L of the call in
// its lambda.
fun <L> one(arg: L): List<L> = listOf(arg)

fun main() {
    val l = buildList {
        flatMap { one(it) }
        add("a")
        println(flatMap { one(it.length) })
    }
    println(l)
}
