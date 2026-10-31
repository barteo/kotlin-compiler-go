import shapes.*

// Uses the library as Kotlin code does: through its metadata.
fun main() {
    val shapes = listOf(Shape.Circle(1.0), Shape.Rect(2.0), Shape.Rect(2.0, 3.0), Shape.Empty)
    for (s in shapes) {
        // Exhaustive without else: the sealed class's subclasses.
        val kind = when (s) {
            is Shape.Circle -> "C"
            is Shape.Rect -> "R"
            Shape.Empty -> "E"
        }
        println(kind + " " + describe(s) + " " + s.area)
    }
    val r = Shape.Rect(2.0, 5.0)
    val (w, h) = r
    println("" + w + " " + h + " " + r.copy(h = 1.0) + " " + (r == Shape.Rect(2.0, 5.0)))
    println(Color.entries.joinToString { it.name + it.hex } + " " + Color.valueOf("GREEN").ordinal)
    println((Meters(1.5) + Meters(2.0)).value)
    println(Meters(1.5).scaled() + " " + Meters(1.0).scaled(unit = "cm") + " " + Meters(3.0).fold({ it + 1 }))
    val box: Box<Any> = Box("abc").map { it.length }
    val (item) = box
    println(item)
    val p = Person("Ann")
    p.visits += 2
    println(p.greet() + " " + p.greet(greeting = "Hi") + " " + p[1] + " " + p(2) + " " + p.visits + " " + p.nick)
    println(Person.SPECIES + " " + Person.of("Bob").name + " " + Registry.add("a", "b", "c"))
    println(format(1.5) + " " + format(2.5, suffix = "m") + " " + format(3.0, "<", ">"))
    println("hey".shout() + " " + "kotlin".initial + " " + lengthOf(null) + " " + lengthOf("four"))
    counter += 5
    println(counter)
    println("" + Ranked(3).higher(7) + " " + Ranked(2.5).higher(1.0) + " " + largest(listOf("pear", "apple", "plum")))
    val upper = Transform { it.uppercase() }
    println(transform("fun", upper) + " " + transform("lambda") { s -> s + "!" })
}
