import android.os.Binder
import android.os.Bundle
import android.os.Parcel
import android.os.PersistableBundle
import android.util.Size
import android.util.SizeF
import android.util.SparseArray
import android.util.SparseBooleanArray
import android.util.SparseIntArray
import android.util.SparseLongArray
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

    roundTrip("builtins", builtins())
    roundTrip("builtin nulls", builtins(nulls = true))
    roundTrip("descriptors", Descriptors(null, listOf(), mapOf("k" to listOf())))
    roundTrip("raw", raw())
    roundTrip("raw nulls", Raw(Plain(1), null, listOf(), listOf(null), null, mapOf(), "s"))
    val box = Box(Inner(1, "one"), listOf(Inner(2, "two")), null)
    roundTrip("box", box)
    roundTrip("nums", Nums(1, arrayOf<Number>(2, 3L, 4.5), "s"))
    roundTrip("nums nulls", Nums<Int, String>(1, null, null))
    roundTrip("raw box", RawBox(Plain(1), listOf(Plain(2), Plain(3))))
    roundTrip("boxes", Boxes(box, null, listOf(box), Unused(5)))

    // newArray makes an array of the class.
    val creator = Inner::class.java.getField("CREATOR").get(null) as Parcelable.Creator<*>
    val array = creator.newArray(2)
    println("newArray: ${array.size} ${array.javaClass.simpleName}")
}

fun builtins(nulls: Boolean = false): Builtins {
    val bundle = Bundle()
    bundle.putString("k", "v")
    val pbundle = PersistableBundle()
    pbundle.putString("p", "q")
    val binder = Binder("b")
    val sba = SparseBooleanArray()
    sba.put(3, true)
    sba.put(1, false)
    val sia = SparseIntArray()
    sia.put(2, 20)
    val sla = SparseLongArray()
    sla.put(5, 50L)
    val sa = SparseArray<String>()
    sa.put(7, "seven")
    sa.put(4, "four")
    val nsa = SparseArray<Inner?>()
    nsa.put(1, Inner(1, "one"))
    nsa.put(2, null)
    val deque = ArrayDeque<String>()
    deque.add("d")
    val jdeque = java.util.ArrayDeque<Long>()
    jdeque.add(8L)
    val conc = java.util.concurrent.ConcurrentHashMap<String, Long>()
    conc["c"] = 9L
    return Builtins(
        StringBuilder("cs"), if (nulls) null else "ncs", bundle, if (nulls) null else bundle, if (nulls) null else pbundle,
        intArrayOf(1, 2), if (nulls) null else intArrayOf(3), longArrayOf(4L), floatArrayOf(1.5f), doubleArrayOf(2.5),
        booleanArrayOf(true, false), byteArrayOf(5), charArrayOf('x'), shortArrayOf(6, 7), if (nulls) null else shortArrayOf(8),
        arrayOf(9L, 10L), if (nulls) null else arrayOf(11, null), arrayOf(Inner(2, "two")), arrayOf(Color.RED, null),
        binder, if (nulls) null else binder, arrayOf(binder), listOf(binder),
        Size(1, 2), if (nulls) null else Size(3, 4), SizeF(1.5f, 2.5f), sba, if (nulls) null else sba,
        sia, if (nulls) null else sia, sla, sa, if (nulls) null else nsa,
        hashSetOf(12), deque, jdeque, sortedSetOf("b", "a"),
        if (nulls) null else java.util.TreeSet(listOf(14, 13)), java.util.TreeMap(mapOf("t" to 15)), sortedMapOf(16 to Inner(3, "three")),
        conc, Marker, if (nulls) null else Marker,
        StringBuilder("sb"), java.util.LinkedList(listOf(17)), Twice,
    )
}

fun raw() = Raw(Plain(1), Plain(2), listOf(Plain(3)), listOf(Plain(4), null), Plain(5), mapOf("k" to Plain(6)), "s")
