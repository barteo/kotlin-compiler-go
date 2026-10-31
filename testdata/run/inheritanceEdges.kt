// More inheritance: bridges for generic interfaces, lambdas in default
// methods, super<I>, and joins of sibling classes.

interface Producer<T> {
    fun produce(): T
    fun twice(): String {
        val f: () -> String = { "" + produce() + produce() }
        return f()
    }
}

class Counter : Producer<Int> {
    var n = 0
    override fun produce(): Int {
        n++
        return n
    }
}

interface Left {
    fun hello(): String = "left"
}

interface Right {
    fun hello(): String = "right"
}

class Both : Left, Right {
    override fun hello(): String = super<Left>.hello() + "+" + super<Right>.hello()
}

interface HasId {
    val id: Int
}

open class Entity(override val id: Int) : HasId {
    override fun toString(): String = "entity" + id
}

class User(id: Int, val name: String) : Entity(id) {
    override fun toString(): String = name + " " + super.toString()
}

class Item(id: Int) : Entity(id) {
    override fun toString(): String = "item" + id
}

class ByLength : java.util.Comparator<String> {
    override fun compare(a: String, b: String): Int = a.length - b.length
}

abstract class Animal {
    abstract val sound: String
    fun speak(): String = sound + "!"
}

class Dog : Animal() {
    override val sound = "woof"
}

class Cat : Animal() {
    override val sound: String
        get() = "meow"
}

fun pet(dog: Boolean): Animal = if (dog) Dog() else Cat()

fun entity(k: Int): Entity = when (k) {
    0 -> User(1, "ann")
    1 -> Item(2)
    else -> Entity(3)
}

fun produceAll(p: Producer<Int>): Int = p.produce() + p.produce()

fun main() {
    val c = Counter()
    println(produceAll(c))
    println(c.twice())
    val p: Producer<Int> = c
    println(p.produce())

    println(Both().hello())
    val left: Left = Both()
    println(left.hello())

    for (k in 0..2) {
        val e = entity(k)
        val h: HasId = e
        println("" + h.id + " " + e)
    }
    println(User(7, "bob").name)

    val words = java.util.ArrayList<String>()
    words.add("ccc")
    words.add("a")
    words.add("bb")
    java.util.Collections.sort(words, ByLength())
    println(words)

    println(pet(true).speak() + " " + pet(false).speak())
    val animal = if (words.size > 2) Dog() else Cat()
    println(animal.sound)
}
