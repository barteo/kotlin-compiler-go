package app

import kotlinx.serialization.InternalSerializationApi
import kotlinx.serialization.KSerializer
import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer
import lib.LibColor
import lib.LibObj
import lib.LibPlain
import lib.LibPoint
import lib.LibShape
import lib.listRoundTrip
import kotlin.reflect.KClass
import kotlin.reflect.KType
import kotlin.reflect.typeOf

val withDefaults = Json { encodeDefaults = true }

// check writes v as JSON, with and without its default values, and to a
// list, and reads it back.
fun <T> check(name: String, s: KSerializer<T>, v: T) {
    val text = Json.Default.encodeToString(s, v)
    val back = Json.Default.decodeFromString(s, text)
    println("$name: $text")
    println("  back: $back ${back == v}")
    println("  defaults: ${withDefaults.encodeToString(s, v)}")
    println("  list: ${listRoundTrip(s, v)}")
}

fun <T> fails(name: String, s: KSerializer<T>, text: String) {
    try {
        println("$name: ${Json.Default.decodeFromString(s, text)}")
    } catch (e: Exception) {
        println("$name: ${e.javaClass.simpleName}: ${e.message?.lines()?.first()}")
    }
}

// A KSerializer<Any?> or a KClass<T>'s KSerializer<T>, for a T : Any, as
// ktor's TypeInfo.serializer().
@OptIn(InternalSerializationApi::class)
fun serializerOf(type: KClass<*>, kotlinType: KType?): KSerializer<out Any?> =
    kotlinType?.let { serializer(it) } ?: type.serializer()

fun main() {
    check("prims", Prims.serializer(), Prims(1, 2L, 3, 4, 'c', 1.5f, 2.5, true, "s"))
    check("nullables", Nullables.serializer(), Nullables(str = null))
    check("nullables2", Nullables.serializer(), Nullables(1, "s", LibPoint(1, 2), listOf("a", null), null))
    check("defaults", Defaults.serializer(), Defaults())
    check("defaults2", Defaults.serializer(), Defaults(1, 2, "r", "m"))
    println(Json.Default.decodeFromString(Defaults.serializer(), """{"a":3,"r":"x","body":"b"}"""))
    check("enums", Enums.serializer(), Enums(Color.GREEN, Mood.SAD, Plain.Y, LibColor.GREEN, LibPlain.B))
    check(
        "collections", Collections.serializer(),
        Collections(
            listOf(1, 2), mutableListOf("a"), arrayListOf(3L), setOf("s"), hashSetOf(4), mapOf("k" to 1), hashMapOf(5 to "v"),
            1 to "one", Triple(1, 2L, "three"), listOf(mapOf("m" to listOf(1, null))), listOf(LibPoint(7)),
        ),
    )
    check("circle", Shape.serializer(), Shape.Circle(1.5))
    check("rect", Shape.serializer(), Shape.Rect(2))
    check("empty", Shape.serializer(), Shape.Empty)
    check("deep", Shape.serializer(), Shape.Nested.Deep("q"))
    check("route", Route.serializer(), Route.Detail("d"))
    check("route object", Route.serializer(), Route.Home)
    check("route nested", Route.serializer(), Page(1))
    check("trip", Trip.serializer(), Trip(Route.Home, Page(2)))
    println("${Route.serializer().descriptor.serialName} ${Route.Nested.serializer().descriptor.serialName} ${Route.serializer() !== Route.serializer()}")
    check("click", Event.serializer(), Event.Click(3))
    check("key", Event.Key.serializer(), Event.Key("k", true))
    check("base", Base.serializer(), Base(5))
    check("derived", Derived.serializer(), Derived(6))
    check("fromPlain", FromPlain.serializer(), FromPlain(8))
    check("singleton", Singleton.serializer(), Singleton)
    check("wrapper", Wrapper.serializer(), Wrapper(Shape.Empty, Event.Click(1), Derived(2), Singleton, LibPoint(3), LibShape.Dot, LibObj))
    check("companion", WithCompanion.serializer(), WithCompanion.make())
    check("inner", Outer.Inner.serializer(), Outer.Inner(9))
    check("hidden", Hidden.serializer(), Hidden(1, "i"))
    check("color", Color.serializer(), Color.GREEN)
    check("stamp", Stamp.serializer(), Stamp(5L))
    check("stamp2", Stamp.serializer(), Stamp(1, 2L))
    println(serializerOf(Prims::class, null).descriptor.serialName)
    println(serializerOf(Prims::class, typeOf<Prims?>()).descriptor.serialName)

    println(Prims.serializer().descriptor)
    println(Defaults.serializer().descriptor)
    println(Derived.serializer().descriptor)
    println(Animal.serializer().descriptor.serialName)
    println(Shape.serializer().descriptor.serialName)
    println(Mood.serializer().descriptor)
    println(serializer<Prims>().descriptor.serialName)
    println(serializer<Color>().descriptor.serialName)

    fails("missing", Prims.serializer(), """{"i":1}""")
    fails("missingBase", Derived.serializer(), """{"extra":1}""")
    fails("unknown", Base.serializer(), """{"id":1,"x":2}""")
    fails("unknownType", Shape.serializer(), """{"type":"nope"}""")
    // As kotlinc's, a $serializer class is synthetic.
    println(Prims.serializer().javaClass.isSynthetic)
    println(log)
}
