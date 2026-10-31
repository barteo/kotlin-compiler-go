// Enum entries, enums from the standard library, and destructuring of
// the standard library's pairs.

enum class Suit { CLUBS, DIAMONDS, HEARTS, SPADES }

fun main() {
    for (s in Suit.entries) {
        print(s.name.lowercase() + " ")
    }
    println()
    println(Suit.entries.size)
    println(Suit.entries[2])
    println(kotlin.time.DurationUnit.SECONDS.name)
    val (a, b) = Pair(1, "one")
    println("" + a + " " + b)
    val (x, y, z) = Triple(1, 2.0, "three")
    println("" + x + y + z)
    val ages = mutableMapOf<String, Int>()
    ages["ann"] = 31
    ages["bob"] = 42
    for ((name, age) in ages) {
        println(name + " " + age)
    }
    for (e in ages.entries) {
        val (name, _) = e
        print(name)
    }
    println()
}
