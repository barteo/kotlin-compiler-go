package app

import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient
import kotlinx.serialization.builtins.nullable
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer
import lib.LibBox
import kotlin.reflect.typeOf

// Generic serializable classes: their $serializer is a class whose
// constructor takes their type arguments' serializers, as are their
// companions' serializer functions and their write$Self.
@Serializable
data class GBox<T>(val value: T)

@Serializable
data class GPage<T>(val items: List<T>, val next: T? = null)

@Serializable
data class GTwo<A, B : Any>(val a: A, val m: Map<String, B>, val b: B?)

// A generic superclass, and subclasses that give it type arguments of
// their own and not.
@Serializable
open class GBase<T>(val t: T) {
    override fun toString() = "GBase($t)"
    override fun equals(other: Any?) = other is GBase<*> && other.javaClass == javaClass && other.toString() == toString()
    override fun hashCode() = toString().hashCode()
}

@Serializable
class GIntBase(val x: Int) : GBase<Int>(1) {
    override fun toString() = "GIntBase($t, $x)"
}

@Serializable
class GSub<U>(val u: U) : GBase<List<U>>(listOf(u)) {
    override fun toString() = "GSub($t, $u)"
}

@Serializable
class GNull(val x: Int) : GBase<Nothing?>(null) {
    override fun toString() = "GNull($t, $x)"
}

@Serializable
sealed class GRes<out T> {
    @Serializable
    data class Ok<T>(val v: T) : GRes<T>()

    @Serializable
    data object Loading : GRes<Nothing>()

    @Serializable
    data class Err(val msg: String) : GRes<Nothing>()

    // A type parameter the sealed class's doesn't give.
    @Serializable
    data class Tagged<A, B>(val a: A, val b: B) : GRes<B>()
}

@Serializable
sealed interface GShape<out T> {
    @Serializable
    data class Circle<T>(val r: T) : GShape<T>

    @Serializable
    data object Empty : GShape<Nothing>
}

@Serializable
abstract class GAbs<T>(val a: T)

@Serializable
data class GTree<T>(val value: T, val children: List<GTree<T>> = emptyList())

@Serializable
data class GCmp<T : Comparable<T>>(val v: T, val list: List<T> = emptyList())

@Serializable
class GWhere<T>(val t: T) where T : CharSequence, T : Comparable<T> {
    override fun toString() = "GWhere($t)"
    override fun equals(other: Any?) = other is GWhere<*> && other.t == t
    override fun hashCode() = t.hashCode()
}

object GOuter {
    @Serializable
    data class Inner<K, V>(val map: Map<K, V>, @Transient val t: Int = 7, val box: GBox<List<V>>? = null)
}

@Serializable
data class GHolder<T : Any>(val m: Map<String, List<T?>>, val p: Pair<T, Int>)

// A class whose properties are of generic classes, of the program's and
// of another module's. Properties named lib and kotlin don't hide the
// packages that the plugin's code names their serializers by.
@Serializable
data class GUser(
    val b: GBox<String>,
    val n: GBox<Int?>?,
    val l: List<GBox<Long>>,
    val r: GRes<String>,
    val two: GTwo<Int, String>,
    val s: GShape<Long>,
    val lib: LibBox<Int>,
    val nested: GBox<LibBox<String>>,
    val kotlin: List<Int> = listOf(1),
)

fun generics() {
    check("gbox", GBox.serializer(Int.serializer()), GBox(1))
    check("gbox nullable", GBox.serializer(String.serializer().nullable), GBox(null))
    check("gpage", GPage.serializer(String.serializer()), GPage(listOf("a", "b")))
    check("gpage2", GPage.serializer(Long.serializer()), GPage(listOf(1L), 2L))
    check("gtwo", GTwo.serializer(Int.serializer(), String.serializer()), GTwo(1, mapOf("k" to "v"), null))
    check("gbase", GBase.serializer(String.serializer()), GBase("s"))
    check("gintbase", GIntBase.serializer(), GIntBase(2))
    check("gsub", GSub.serializer(Int.serializer()), GSub(3))
    // As kotlinc's, GNull writes its superclass's t with a NothingSerializer,
    // which can't write its null: only its descriptor.
    println(GNull.serializer().descriptor)
    val res = GRes.serializer(String.serializer())
    check("gres ok", res, GRes.Ok("v"))
    check("gres loading", res, GRes.Loading)
    check("gres err", res, GRes.Err("e"))
    checkJson("gres ok int", GRes.serializer(Int.serializer()), GRes.Ok(5))
    check("gshape", GShape.serializer(Long.serializer()), GShape.Circle(6L))
    check("gshape empty", GShape.serializer(Long.serializer()), GShape.Empty)
    check("gtree", GTree.serializer(Int.serializer()), GTree(1, listOf(GTree(2), GTree(3, listOf(GTree(4))))))
    check("gcmp", GCmp.serializer(String.serializer()), GCmp("a", listOf("b")))
    check("gwhere", GWhere.serializer(String.serializer()), GWhere("w"))
    check("ginner", GOuter.Inner.serializer(String.serializer(), Int.serializer()), GOuter.Inner(mapOf("x" to 1), box = GBox(listOf(5))))
    check("libbox", LibBox.serializer(Int.serializer()), LibBox(1))
    check("gholder", GHolder.serializer(Int.serializer()), GHolder(mapOf("k" to listOf(1, null)), 2 to 3))
    check(
        "guser", GUser.serializer(),
        GUser(
            GBox("b"), GBox(null), listOf(GBox(1L)), GRes.Ok("r"), GTwo(1, mapOf("k" to "v"), "b"),
            GShape.Circle(2L), LibBox(3, "t"), GBox(LibBox("n")),
        ),
    )
    println(GBox.serializer(Int.serializer()).descriptor)
    println(GPage.serializer(String.serializer()).descriptor)
    println(GSub.serializer(Int.serializer()).descriptor)
    println(GUser.serializer().descriptor)
    println(GRes.serializer(String.serializer()).descriptor.serialName)
    println(GAbs.serializer(Int.serializer()).descriptor.let { "${it.serialName} ${it.kind}" })
    // Serializers that the program's inline functions and reflection find.
    println(serializer<GBox<String>>().descriptor)
    println(serializer<GPage<GBox<Int>>>().descriptor.getElementDescriptor(0).getElementDescriptor(0))
    println(serializerOf(GBox::class, typeOf<GBox<Int>>()).descriptor)
    println(serialNameOf<GTree<String>>() + " " + encoded(GBox(listOf(1))) + " " + encoded(LibBox(GBox(2))))
    println(Json.Default.decodeFromString<GPage<String>>("""{"items":["x"]}"""))
    // A missing property, whose exception the synthetic constructor
    // throws with the class's cached descriptor.
    fails("gmissing", GBox.serializer(Int.serializer()), """{}""")
    fails("gmissingBase", GSub.serializer(Int.serializer()), """{"u":1}""")
    fails("gmissingTwo", GTwo.serializer(Int.serializer(), Int.serializer()), """{"a":1}""")
    println(GBox.serializer(Int.serializer()).javaClass.isSynthetic)
}
