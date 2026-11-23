// Class literals of kotlin's types by their qualified names.
fun main() {
    println(kotlin.String::class.java)
    println(kotlin.Int::class.javaObjectType)
    println(kotlin.Int::class.javaPrimitiveType)
    println(kotlin.Any::class.java)
    println(kotlin.Unit::class.java)
    val kotlin = "shadow"
    println(kotlin.length)
}
