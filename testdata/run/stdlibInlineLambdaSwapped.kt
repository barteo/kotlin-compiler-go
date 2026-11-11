// kotlinc compiles firstNotNullOf's transform(next()) as next(); aload
// transform; swap; invoke, which inlining the lambda drops the swap of.
class Code(val raw: String?)

fun first(codes: List<Code>) = codes.firstNotNullOf { it.raw }
fun firstSafe(codes: List<Code>?) = codes?.takeIf { it.size == 1 }?.firstNotNullOf { it.raw }
fun firstOfSequence(codes: Sequence<Code>) = codes.firstNotNullOf { it.raw?.uppercase() }

fun main() {
    println(first(listOf(Code(null), Code("a"))))
    println(firstSafe(listOf(Code("b"))))
    println(firstSafe(listOf(Code("b"), Code("c"))))
    println(firstOfSequence(sequenceOf(Code(null), Code("d"))))
    try {
        first(listOf(Code(null)))
    } catch (e: NoSuchElementException) {
        println("none")
    }
}
