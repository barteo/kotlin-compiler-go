// Arrays' toString and equals, Any?.toString on implicit nullable
// receivers and value classes, references to builtin members of implicit
// receivers, member extensions for value classes of one underlying type,
// and extensions called on null.

@JvmInline
value class Id(val id: String)

@JvmInline
value class Name(val name: String)

fun Id?.show() = toString()
fun Id?.template() = "$this"
fun Any?.describe() = toString()
fun Id.test() = "Id " + id
fun Id?.test() = "null Id"

class Outer {
    fun Id.ext() = "Id.ext " + id
    fun Name.ext() = "Name.ext " + name
    fun run() = Id("a").ext() + ", " + Name("b").ext()
}

fun main() {
    val chars = "abc".toCharArray()
    println(chars.toString().startsWith("[C@"))
    println(chars.equals(chars))
    println(intArrayOf(1).equals(intArrayOf(1)))
    println(arrayOf("x").toString() != "x")

    println((null as Id?).show())
    println(Id("x").show())
    println(Id("y").template())
    println((null as Any?).describe())
    println(1.describe())

    var state = 0
    println((state++)::toString.name)
    println(with(state++) { ::toString.name })
    println(with(42) { ::toString }())
    println(state)

    println(Outer().run())
    println(null.test())
    println(Id("z").test())
}
