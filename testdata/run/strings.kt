fun greet(name: String): String = "Hello, $name!"

fun describe(n: Int): String {
    val parity = if (n % 2 == 0) "even" else "odd"
    return "$n is $parity and its square is ${n * n}"
}

fun main() {
    println(greet("Kotlin"))
    println(describe(7))
    val s = "Compiler"
    println(s.length)
    println(s.uppercase())
    println(s.lowercase())
    println(s.substring(3))
    println(s.substring(0, 3))
    println("".isEmpty())
    println("abc" + 1 + 2)
    println("pi is about " + 3.14)
    println("t" + true)
    var acc = ""
    for (i in 1..5) {
        acc += i
    }
    println(acc)
    println("escapes: \t|\"quoted\"|\\|\$notTemplate|A")
    println("123".toInt() + 1)
    println(42.toString() + "!")
    println("a" < "b")
    println("abc" == "ab" + "c")
    val long = 9000000000L
    println("long=$long double=${1.5 * 2}")
}
