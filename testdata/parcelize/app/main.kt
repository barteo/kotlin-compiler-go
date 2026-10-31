import android.os.Parcel
import android.os.Parcelable
import java.math.BigDecimal
import lib.LibOpen
import lib.LibPoint
import p.*

// roundTrip writes x with writeParcelable, which calls its writeToParcel,
// and reads it back with readParcelable, which uses its CREATOR.
fun <T : Parcelable> roundTrip(name: String, x: T) {
    val parcel = Parcel()
    parcel.writeParcelable(x, 0)
    println("$name: ${parcel.log()}")
    val back = parcel.readParcelable<T>(x.javaClass.classLoader)
    println("  read: $back")
    println("  equal: ${back == x}, describeContents: ${x.describeContents()}")
}

fun main() {
    roundTrip("primitives", Primitives("s", "ns", 1, 2, 3L, 4L, true, false, 1.5, 2.5, 3.5f, 4.5f, 'c', 'd', 5, 6, 7, 8))
    roundTrip("nulls", Primitives("s", null, 1, null, 3L, null, false, null, 1.5, null, 3.5f, null, 'c', null, 5, null, 7, null))
    val inner = Inner(1, "one")
    roundTrip(
        "nested",
        Nested(Color.GREEN, Color.BLUE, inner, Inner(2, "two"), Empty, Empty, Shape.Circle(1.0), Shape.Dot, Shape.Circle(2.0),
            Money(100, "EUR"), Money(5, "USD"), BigDecimal("1.50"), BigDecimal("2"), LibPoint(1, 2), LibPoint(3, 4), LibOpen("open"),
            Base("base"), Outer.Deep(9)),
    )
    roundTrip(
        "nested nulls",
        Nested(Color.RED, null, inner, null, Empty, null, Shape.Dot, null, Shape.Circle(2.0), Money(1, "CZK"), null,
            BigDecimal("0.1"), null, LibPoint(0, 0), null, LibOpen("x"), Base("y"), Outer.Deep(0)),
    )
    roundTrip(
        "collections",
        Collections(
            listOf("a", "b"), listOf("c"), listOf("d", null), arrayListOf("e"), listOf(inner, Inner(3, "three")), listOf(inner),
            listOf(1, null, 3), mutableListOf(4L, 5L), setOf("x", "y"), setOf(Color.BLUE, Color.RED), mapOf("k" to inner),
            mapOf(1 to 2L), hashMapOf("h" to 1), listOf(listOf(1L, 2L), listOf()), mapOf("m" to listOf(inner, null)),
            listOf(Shape.Dot, Shape.Circle(3.0)),
        ),
    )
    roundTrip(
        "empty collections",
        Collections(
            listOf(), null, listOf(), arrayListOf(), listOf(), null, listOf(), mutableListOf(), setOf(), setOf(), mapOf(), null,
            hashMapOf(), listOf(), mapOf(), listOf(),
        ),
    )
    roundTrip("object", Empty)
    roundTrip("sealed object", Shape.Dot)
    roundTrip("open", Base("b"))
    roundTrip("described", Described(3))
    roundTrip("deep", Outer.Deep(42))

    roundTrip("enum", Level.HIGH)
    roundTrip("enum with bodies", Tone.LOUD)
    roundTrip("sealed subclass", Step.Go(3, Level.LOW))
    roundTrip("sealed object", Step.Stop)
    roundTrip("nested sealed", Step.Nest(inner, Step.Nest.Kind.Named("n")))
    roundTrip("top-level sealed subclass", Jump(4))
    roundTrip("sealed interface", Edit("e", arrayOf("a", "b"), null))
    roundTrip(
        "modes",
        Modes(Level.LOW, null, Tone.SOFT, Tone.LOUD, Jump(5), Step.Go(6, Level.HIGH), Step.Stop, Edit("f", arrayOf(), arrayOf("c")), null),
    )
    println("tone: ${Tone.LOUD.shout()} ${Tone.LOUD.code}")

    // newArray makes an array of the class.
    val creator = Inner::class.java.getField("CREATOR").get(null) as Parcelable.Creator<*>
    val array = creator.newArray(2)
    println("newArray: ${array.size} ${array.javaClass.simpleName}")
}
