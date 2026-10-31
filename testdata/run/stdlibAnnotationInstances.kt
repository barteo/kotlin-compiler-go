// Instances of annotation classes, A(x), as kotlinc creates them: of a
// class that implements the annotation, equal to the JVM's instances.

import kotlin.reflect.KClass

enum class Color { RED, GREEN }

annotation class Tag(val name: String, val weight: Int = 1)

annotation class Spec(
    val tags: Array<Tag>,
    val ids: IntArray,
    val type: KClass<*>,
    val primary: Tag,
    val color: Color = Color.GREEN,
    val ratio: Double = 0.5,
    val flag: Boolean = false,
    val letter: Char = 'x',
    val big: Long = 1L shl 40,
    val types: Array<KClass<*>> = arrayOf(String::class, Int::class),
)

annotation class Marker

class Outer {
    annotation class Nested(val value: String)
}

@Spec(arrayOf(Tag("a"), Tag("b", 2)), intArrayOf(1, 2), String::class, Tag("p"))
class Annotated

fun spec() = Spec(arrayOf(Tag("a"), Tag("b", 2)), intArrayOf(1, 2), String::class, Tag("p"))

fun main() {
    val s = spec()
    println(s)
    println(s == spec())
    println(s.hashCode() == spec().hashCode())
    val reflected = Annotated::class.java.getAnnotation(Spec::class.java)
    println(s == reflected)
    println(reflected == s)
    println(s.hashCode() == reflected.hashCode())
    println(s.tags[1].weight + s.ids[1] + s.big.toInt())
    println(s.color.name + " " + s.letter + " " + s.flag + " " + s.ratio)
    println(Tag("a") == Tag("a", 1))
    println(Tag("a") == Tag("a", 2))
    println(Marker())
    println(Marker() == Marker())
    println(Marker().hashCode())
    println(Outer.Nested("n"))
    val any: Any = Tag("t")
    println(any is Tag)
    println((any as Annotation).annotationClass == Tag::class)
}
