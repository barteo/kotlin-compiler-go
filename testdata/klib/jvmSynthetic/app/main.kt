import lib.*

fun main() {
    println(withOps { "a" eq count() })
    println(plusOne(2))
    println(answer)
    val b = Box("hi")
    b.value = "there"
    println(b.shout())
    println(typeName<Box>())
}
