import app.Base
import app.Color
import app.Event
import app.Framed
import app.GBase
import app.GBox
import app.GPage
import app.GRes
import app.Label
import app.Margin
import app.Outer
import app.Page
import app.Painted
import app.Rgb
import app.Pet
import app.Prims
import app.Route
import app.Shape
import app.WithCompanion
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer

// Serializable classes that kotlinc compiles against kc's: their
// serializers use kc's $serializer objects and serializer() functions,
// and a subclass's its superclass's synthetic constructor and write$Self.
@Serializable
data class Holder(val p: Prims, val c: Color, val s: Shape, val e: Event, val w: WithCompanion, val inner: Outer.Inner, val r: Route)

@Serializable
class Sub(val more: Int) : Base(2) {
    override fun toString() = "Sub($id, $tag, $more)"
}

fun main() {
    val h = Holder(Prims(1, 2L, 3, 4, 'c', 1.5f, 2.5, true, "s"), Color.GREEN, Shape.Rect(2, 3), Event.Key("k"), WithCompanion(1), Outer.Inner(2), Page(3))
    val text = Json.encodeToString(Holder.serializer(), h)
    println("use: $text")
    println("  back: ${Json.decodeFromString(Holder.serializer(), text) == h}")
    val sub = Json.encodeToString(Sub.serializer(), Sub(3))
    println("sub: $sub -> ${Json.decodeFromString(Sub.serializer(), sub)}")
    println("sub: ${Json.decodeFromString(Sub.serializer(), """{"id":4,"more":5}""")}")
    val m = Json.encodeToString(Measured.serializer(), Measured(Margin(4), Label("m")))
    println("measured: $m -> ${Json.decodeFromString(Measured.serializer(), m)}")
    val mat = Json.encodeToString(Matted.serializer(), Matted(6))
    println("matted: $mat -> ${Json.decodeFromString(Matted.serializer(), mat)}")
    println("matted: ${Json.decodeFromString(Matted.serializer(), """{"w":7}""")}")
    val g = Json.encodeToString(Gallery.serializer(), Gallery(listOf(Painted(Rgb(7uL), name = "g"))))
    println("gallery: $g -> ${Json.decodeFromString(Gallery.serializer(), g)}")
    val pets = Json.encodeToString(ListSerializer(Pet.serializer()), listOf(Pet.Cat(1), Pet.Wild.Fox(2)))
    println("pets: $pets -> ${Json.decodeFromString(ListSerializer(Pet.serializer()), pets)}")
    println(serializer<Prims>().descriptor.serialName)
    // kc's generic classes: their companions' serializer functions, their
    // $serializer classes, which kotlinc's intrinsics construct, and a
    // generic superclass's write$Self and synthetic constructor.
    println("gbox: ${Json.encodeToString(GBox.serializer(Int.serializer()), GBox(1))}")
    println("gpage: ${Json.encodeToString(GPage(listOf(1), 2))} ${Json.decodeFromString<GPage<Int>>("""{"items":[3]}""")}")
    val gh = GHold(GBox("s"), GPage(listOf(GBox(1L))), GRes.Ok(2))
    val ght = Json.encodeToString(GHold.serializer(), gh)
    println("ghold: $ght -> ${Json.decodeFromString(GHold.serializer(), ght) == gh}")
    val gs = Json.encodeToString(GUseSub.serializer(), GUseSub(4))
    println("gusesub: $gs -> ${Json.decodeFromString(GUseSub.serializer(), gs)}")
    val gu = Json.encodeToString(GUseSub2.serializer(Long.serializer()), GUseSub2(5L))
    println("gusesub2: $gu -> ${Json.decodeFromString(GUseSub2.serializer(Long.serializer()), gu)}")
    println(serializer<GBox<String>>().descriptor)
}

@Serializable
data class GHold(val b: GBox<String>, val p: GPage<GBox<Long>>, val r: GRes<Int>)

@Serializable
class GUseSub(val more: Int) : GBase<String>("s") {
    override fun toString() = "GUseSub($t, $more)"
}

@Serializable
class GUseSub2<V>(val v: V) : GBase<V>(v) {
    override fun toString() = "GUseSub2($t, $v)"
}

@Serializable
data class Gallery(val items: List<Painted>)

// kc's value classes' serializers, and the synthetic constructor of a
// class with a property of a value class type, through its marker.
@Serializable
data class Measured(val m: Margin, val l: Label?)

@Serializable
class Matted(val w: Int) : Framed() {
    override fun toString() = "Matted($frame, $w)"
}
