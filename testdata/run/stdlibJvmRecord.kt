// kotlinc: -jvm-target 17
// A @JvmRecord data class is a record, as kotlinc compiles it: it extends
// java.lang.Record, its Record attribute has its constructor's properties,
// with their generic signatures and the annotations Java applies to record
// components, and their accessors are named as them, x(), with bridges
// for the getters of the interface properties they override.

import java.lang.annotation.ElementType
import java.lang.annotation.Retention
import java.lang.annotation.RetentionPolicy

@java.lang.annotation.Target(ElementType.RECORD_COMPONENT, ElementType.FIELD)
@Retention(RetentionPolicy.RUNTIME)
annotation class Component

@Target(AnnotationTarget.FIELD)
annotation class FieldOnly

annotation class Anywhere

interface Named {
    val name: String
    val label: String get() = "label"
}

@JvmRecord
data class Point(val x: Int, val y: Int)

@JvmRecord
data class Box<T>(val value: T, val items: List<String>, val point: Point?)

@JvmRecord
data class Person(
    override val name: String,
    @Component val age: Int,
    @FieldOnly val email: String,
    @field:Anywhere val nick: String,
    @Anywhere val city: String,
) : Named {
    override val label: String get() = "person"
    val upper: String get() = name.uppercase()
    fun greet() = "hi $name"

    companion object {
        fun of(name: String) = Person(name, 30, "$name@example.com", name.lowercase(), "Oslo")
    }
}

@JvmRecord
data class Sized(override val size: Int) : Collection<Int> {
    override fun iterator(): Iterator<Int> = (0 until size).iterator()
    override fun contains(element: Int) = element in 0 until size
    override fun containsAll(elements: Collection<Int>) = elements.all { it in this }
    override fun isEmpty() = size == 0
}

fun describe(c: Class<*>) {
    println("${c.simpleName}: record=${c.isRecord} super=${c.superclass.name}")
    for (rc in c.recordComponents) {
        val anns = rc.annotations.map { it.annotationClass.java.simpleName }.sorted()
        println("  ${rc.name}: ${rc.type.simpleName} sig=${rc.genericSignature} accessor=${rc.accessor.name} $anns")
    }
    val getters = c.declaredMethods.filter { it.name.startsWith("get") && it.parameterCount == 0 }
        .map { it.name + if (it.isBridge) "(bridge)" else "" }.sorted()
    println("  getters: $getters")
}

fun main() {
    val p = Point(1, 2)
    println(p)
    println(p == Point(1, 2))
    println(p.hashCode() == Point(1, 2).hashCode())
    println(p.copy(y = 5))
    val (x, y) = p
    println(x + y)
    val r: Record = p
    println(r is Point)
    println(Point::class.java.getMethod("x").invoke(p))

    val b = Box("v", listOf("a", "b"), null)
    println(b.value + b.items + b.point)

    val person = Person.of("Ann")
    val named: Named = person
    println(named.name + " " + named.label + " " + person.upper + " " + person.greet())
    println(Named::class.java.getMethod("getName").invoke(person))

    val s = Sized(3)
    val c: Collection<Int> = s
    println("${s.size} ${c.size} ${c.toList()}")

    describe(Point::class.java)
    describe(Box::class.java)
    describe(Person::class.java)
    describe(Sized::class.java)
}
