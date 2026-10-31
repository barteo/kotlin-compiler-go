// The JVM shape of value classes, as kotlinc writes them, where it needs
// the standard library: constructors that take a value class are private,
// and callers call one that takes a marker, a kotlin.jvm.internal class.

@JvmInline
value class Meters(val value: Double) {
    operator fun plus(other: Meters): Meters = Meters(value + other.value)

    fun format(): String = "%.1f m".format(value)
}

@JvmInline
value class Tags(val tags: List<String>) {
    val size: Int
        get() = tags.size
}

open class Route(val length: Meters, val tags: Tags = Tags(listOf())) {
    constructor(name: String, length: Meters) : this(length, Tags(listOf(name)))
}

class Walk(length: Meters) : Route(length)

interface Taggable {
    val tags: Map<String, Any>
}

// As kotlinpoet's TagMap: a private constructor, and an invoke operator
// in the companion that callers call instead.
@JvmInline
value class TagMap private constructor(override val tags: Map<String, Any>) : Taggable {
    companion object {
        operator fun invoke(tags: Map<String, Any>): TagMap = TagMap(tags.toMap())
    }
}

fun Map<String, Any>.buildTagMap(): TagMap = TagMap(this)

class Spec(val name: String, private val tagMap: TagMap = mapOf<String, Any>().buildTagMap()) : Taggable by tagMap

fun total(routes: List<Route>): Meters = routes.map { it.length }.fold(Meters(0.0)) { a, b -> a + b }

fun signatures(c: Class<*>) {
    println(c.name + ":")
    val methods = c.declaredMethods.filter { !it.isSynthetic && java.lang.reflect.Modifier.isPublic(it.modifiers) }
        .map { it.name + it.parameterTypes.map { p -> p.simpleName } + ": " + it.returnType.simpleName }
    val ctors = c.declaredConstructors.map {
        "<init>" + it.parameterTypes.map { p -> p.simpleName } +
            (if (java.lang.reflect.Modifier.isPublic(it.modifiers)) " public" else " private") +
            (if (it.isSynthetic) " synthetic" else "")
    }
    for (s in (methods + ctors).sorted()) {
        println("  " + s)
    }
}

fun main() {
    val r = Route(Meters(1.5))
    val n = Route("park", Meters(2.0))
    val w = Walk(Meters(0.5))
    println(total(listOf(r, n, w)).format())
    println(n.tags.size)
    println(r.tags)
    println(listOf(Meters(1.0), Meters(2.0)).map { it.value * 2 })
    val ms: List<Meters> = listOf(Meters(3.0))
    println(ms.first() + Meters(1.0))
    val t = mapOf<String, Any>("k" to 1).buildTagMap()
    println(t.tags["k"])
    println(Spec("s", t).tags)
    println(Spec("empty").tags.isEmpty())
    signatures(Meters::class.java)
    signatures(Tags::class.java)
    signatures(Route::class.java)
    signatures(Walk::class.java)
}
