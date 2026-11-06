import app.Base
import app.Color
import app.Event
import app.Framed
import app.Label
import app.Margin
import app.Outer
import app.Page
import app.Prims
import app.Route
import app.Shape
import app.WithCompanion
import kotlinx.serialization.Serializable
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
    println(serializer<Prims>().descriptor.serialName)
}

// kc's value classes' serializers, and the synthetic constructor of a
// class with a property of a value class type, through its marker.
@Serializable
data class Measured(val m: Margin, val l: Label?)

@Serializable
class Matted(val w: Int) : Framed() {
    override fun toString() = "Matted($frame, $w)"
}
