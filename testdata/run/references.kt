class Person(val name: String, var age: Int) {
    fun greet(other: String): String = "$name greets $other"
    fun older(): Person = Person(name, age + 1)
    fun selfRef(): () -> String = this::describe
    fun describe(): String = "$name ($age)"
    fun ages(people: Array<Person>): String = joinAll(people, ::label)
    private fun label(p: Person): String = "${p.name}:${p.age - age}"
}

class Box<T>(val value: T) {
    fun get(): T = value
}

class Widget(val id: Int) {
    companion object {
        fun make(): Widget = Widget(0)
    }
}

object Registry {
    fun find(id: Int): String = "item$id"
}

var calls = 0
val greeting = "hi"

fun double(x: Int): Int = x * 2
fun triple(x: Long): Long = x * 3
fun pick(x: Int): String = "int $x"
fun pick(s: String): String = "string $s"
fun withDefault(x: Int, y: Int = 10): Int = x + y
fun sayHello() = println("hello")
fun count(vararg xs: Int): Int = xs.size
fun String.shout(): String = uppercase() + "!"

fun applyInt(f: (Int) -> Int, x: Int): Int = f(x)
fun applyStr(f: (String) -> String, x: String): String = f(x)
fun <T, R> transform(x: T, f: (T) -> R): R = f(x)
fun <T> joinAll(xs: Array<T>, f: (T) -> String): String {
    var s = ""
    for (x in xs) s += (if (s.isEmpty()) "" else " ") + f(x)
    return s
}
fun twice(f: () -> Unit) {
    f()
    f()
}
fun receiverCall(f: String.() -> Int, s: String): Int = s.f()

fun byLength(a: String, b: String): Int = a.length - b.length

fun next(): Person {
    calls++
    return Person("N$calls", calls)
}

fun main() {
    println(applyInt(::double, 21))
    val f = ::double
    println(f(5))
    val t = ::triple
    println(t(2L))
    val sp: (Int) -> String = ::pick
    val ss: (String) -> String = ::pick
    println(sp(1) + ", " + ss("a"))
    println(applyInt(::withDefault, 1))
    twice(::sayHello)
    val two: (Int, Int) -> Int = ::count
    val arr: (IntArray) -> Int = ::count
    println("${two(1, 2)} ${arr(intArrayOf(1, 2, 3))}")
    println(applyStr(String::shout, "hey"))
    println(transform("abc", String::length))
    println(transform(3, ::double))

    val p = Person("Ann", 30)
    val name: (Person) -> String = Person::name
    println(name(p))
    val greet = p::greet
    println(greet("Bob"))
    println(joinAll(arrayOf(p, Person("Joe", 4)), Person::describe))
    println(transform(p, Person::older).age)
    println(p.selfRef()())
    println(p.ages(arrayOf(Person("Kid", 3), Person("Old", 80))))
    val mk = ::Person
    println(mk("Zed", 9).describe())
    val ages = Person::age
    p.age = 31
    println(ages(p))

    val box = Box(42)
    val get = box::get
    println(get())
    val unboxed: (Box<String>) -> String = Box<String>::get
    println(unboxed(Box("boxed")))

    val newWidget = ::Widget
    println(newWidget(5).id)
    val noArgs: (Int) -> Widget = ::Widget
    println(noArgs(6).id)
    val make = Widget.Companion::make
    println(make().id)
    val find = Registry::find
    println(find(7))
    val g = ::greeting
    println(g())

    val parse: (String) -> Int = Integer::parseInt
    println(parse("42") + 1)
    val len: (String) -> Int = String::length
    println(len("hello"))
    println(receiverCall(String::length, "four"))
    val upper: (String) -> String = String::uppercase
    println(upper("up"))

    // A bound receiver is evaluated once, when the reference is made.
    val bound = next()::describe
    println(calls)
    println(bound())
    println(bound())
    println(calls)

    val r = Runnable(::sayHello)
    r.run()
    val list = java.util.ArrayList<String>()
    list.add("bb")
    list.add("ccc")
    list.add("a")
    java.util.Collections.sort(list, ::byLength)
    println(list)
    val people = java.util.ArrayList<Person>()
    people.add(Person("Y", 2))
    people.add(Person("X", 1))
    java.util.Collections.sort(people, java.util.Comparator.comparingInt(Person::age))
    println(people[0].name)
}
