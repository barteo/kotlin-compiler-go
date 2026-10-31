package app

import kotlinx.serialization.Required
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient
import lib.LibColor
import lib.LibObj
import lib.LibPlain
import lib.LibPoint
import lib.LibShape

val log = mutableListOf<String>()

@Serializable
data class Prims(val i: Int, val l: Long, val s: Short, val b: Byte, val c: Char, val f: Float, val d: Double, val z: Boolean, val str: String)

@Serializable
data class Nullables(
    val i: Int? = null,
    val str: String?,
    val p: LibPoint? = null,
    val list: List<String?>? = null,
    val e: Color? = Color.RED,
)

// Default values that use other properties, properties declared in the
// body, transient ones and init blocks, which the synthetic constructor
// runs in order.
@Serializable
class Defaults(val a: Int = 5, val b: Int = a * 2, @Required val r: String = "req", @SerialName("renamed") val n: String = "n") {
    var body: String = "body$a"

    @Transient
    val t: Int = b + 1
    val computed: Int get() = a + 1
    val lazyV by lazy { a * 100 }

    init {
        log.add("init $a $b $body $t")
    }

    override fun toString() = "Defaults(a=$a, b=$b, r=$r, n=$n, body=$body, t=$t, computed=$computed, lazyV=$lazyV)"
}

@Serializable
enum class Color { RED, @SerialName("vert") GREEN, BLUE }

@Serializable
@SerialName("Mood")
enum class Mood { HAPPY, SAD }

enum class Plain { X, Y }

@Serializable
data class Enums(val c: Color, val m: Mood, val p: Plain, val lc: LibColor, val lp: LibPlain)

@Serializable
data class Collections(
    val list: List<Int>,
    val ml: MutableList<String>,
    val al: ArrayList<Long>,
    val set: Set<String>,
    val hs: HashSet<Int>,
    val map: Map<String, Int>,
    val hm: HashMap<Int, String>,
    val pair: Pair<Int, String>,
    val triple: Triple<Int, Long, String>,
    val nested: List<Map<String, List<Int?>>>,
    val points: List<LibPoint>,
)

@Serializable
sealed class Shape {
    @Serializable
    @SerialName("circle")
    data class Circle(val r: Double) : Shape()

    @Serializable
    data class Rect(val w: Int, val h: Int = w) : Shape()

    @Serializable
    data object Empty : Shape()

    @Serializable
    sealed class Nested : Shape() {
        @Serializable
        data class Deep(val q: String) : Nested()
    }
}

// A sealed class with a property, which its subclasses' serializers
// write first.
@Serializable
sealed class Event(val id: Int) {
    @Serializable
    data class Click(val x: Int) : Event(1)

    @Serializable
    class Key(val code: String, val shift: Boolean = false) : Event(2) {
        override fun toString() = "Key($id, $code, $shift)"
    }
}

@Serializable
open class Base(val id: Int, val tag: String = "t") {
    override fun toString() = "Base($id, $tag)"
}

@Serializable
class Derived(val extra: Long) : Base(1) {
    override fun toString() = "Derived($id, $tag, $extra)"
}

open class NotSerializable {
    val z = 3
}

@Serializable
data class FromPlain(val w: Int) : NotSerializable()

@Serializable
abstract class Animal {
    abstract val name: String
}

@Serializable
data object Singleton {
    val x = 1
}

interface Iface

@Serializable
data class Wrapper(
    val shape: Shape,
    val event: Event,
    val base: Base,
    val o: Singleton,
    val lp: LibPoint,
    val ls: LibShape,
    val lo: LibObj,
    val iface: Iface? = null,
)

@Serializable
data class WithCompanion(val v: Int) {
    companion object {
        const val K = 7
        fun make() = WithCompanion(K)
    }
}

class Outer {
    @Serializable
    data class Inner(val q: Int)
}

@Serializable
data class Hidden(private val secret: Int, internal val inner: String) {
    fun secret() = secret
}

// Functions of a class's name in its package, as ktor's GMTDate has: the
// synthetic constructor's call, by its qualified name, constructs it.
@Serializable
data class Stamp(val seconds: Int, val color: Color, val ts: Long)

fun Stamp(ts: Long? = null): Stamp = Stamp(0, Color.RED, ts ?: 0L)

fun Stamp(seconds: Int, ts: Long): Stamp = Stamp(seconds, Color.BLUE, ts)

// A sealed interface's companion makes a SealedClassSerializer of its
// subclasses, through sealed subinterfaces too, each time it is asked.
@Serializable
sealed interface Route {
    @Serializable
    data object Home : Route

    @Serializable
    @SerialName("detail")
    data class Detail(val id: String) : Route

    @Serializable
    sealed interface Nested : Route
}

@Serializable
data class Page(val n: Int) : Route.Nested

@Serializable
data class Trip(val from: Route, val to: Route.Nested?)
