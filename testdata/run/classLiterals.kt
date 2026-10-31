package lits

open class Shape
class Circle : Shape()
class Outer {
    class Nested
}
object Registry
interface Named
data class Box<T>(val value: T)

fun nameOf(c: Class<*>): String = c.name

fun describe(s: Shape): String = s::class.java.simpleName

fun main() {
    println(Circle::class.java.name)
    println(Shape::class.java.simpleName)
    println(Outer.Nested::class.java.name)
    println(lits.Outer::class.java.name)
    println(Registry::class.java.simpleName)
    println(Named::class.java.isInterface)
    println(Box::class.java.simpleName)
    println(nameOf(String::class.java))
    println(nameOf(Any::class.java))
    println(Int::class.java)
    println(Long::class.java)
    println(Boolean::class.java)
    println(Char::class.java)
    println(IntArray::class.java.simpleName)
    println(Array<String>::class.java.simpleName)
    println(java.util.ArrayList::class.java.name)
    println(Runnable::class.java.name)

    val shapes = arrayOf(Shape(), Circle())
    for (s in shapes) println(describe(s))
    println(Shape::class.java.isInstance(shapes[1]))
    println(Circle::class.java.isInstance(shapes[0]))
    println(shapes[1]::class.java == Circle::class.java)
    println(shapes[0]::class.java === Shape::class.java)
    val n = 42
    println(n::class.java)
    val boxed: Any = n
    println(boxed::class.java.name)
    println("text"::class.java.name)
    val b = Box(1)
    println(b::class.java.simpleName)
    println(Circle::class.java.superclass.simpleName)
}
