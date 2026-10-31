import app.Base
import app.Color
import app.Event
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
    println(serializer<Prims>().descriptor.serialName)
}
