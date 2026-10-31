data class Person(val name: String, val age: Int)

fun square(x: Int) = x * x
fun isEven(x: Int) = x % 2 == 0
fun add(a: Int, b: Int) = a + b
fun label(x: Int): String = "#$x"

class Counter {
    var n = 0
    fun bump(x: Int) {
        n += x
    }
}

fun main() {
    val xs = listOf(1, 2, 3, 4)
    println(xs.map(::square))
    println(xs.filter(::isEven))
    println(xs.fold(0, ::add))
    println(xs.map(::label).joinToString(""))
    println(listOf("a", "bbb", "cc").map(String::length))
    println(listOf("a", "", "c").filter(String::isNotEmpty))
    val people = listOf(Person("Bo", 40), Person("Al", 20))
    println(people.sortedBy(Person::age).map(Person::name))
    println(people.map(Person::name).joinToString())
    println(people.maxByOrNull(Person::age)?.name)
    println(people.any(Person::name.let { n -> { p: Person -> n(p) == "Al" } }))
    xs.forEach(::println)
    val c = Counter()
    xs.forEach(c::bump)
    println(c.n)
    println(listOf("1", "2").map(String::toInt).sum())
    println(xs.map(Int::toString))
    println(listOf(3, 1, 2).sortedWith(compareBy(::square)))
    val mk: (String, Int) -> Person = ::Person
    println(mk("Cy", 5))
    println(listOf("x", "y").map { Person(it, 1) }.map(Person::component1))
    println(xs.associateBy(::label))
    println(listOf(listOf(1), listOf(2, 3)).flatMap { it }.map(::square))
}
