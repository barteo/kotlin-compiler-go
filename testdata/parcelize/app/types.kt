package p

import android.os.Parcelable
import kotlinx.parcelize.IgnoredOnParcel
import kotlinx.parcelize.Parcelize
import java.math.BigDecimal
import lib.LibOpen
import lib.LibPoint

enum class Color { RED, GREEN, BLUE }

@Parcelize
data class Inner(val x: Int, val name: String) : Parcelable

@Parcelize
object Empty : Parcelable {
    override fun toString() = "Empty"
}

sealed class Shape : Parcelable {
    @Parcelize
    data class Circle(val r: Double) : Shape()

    @Parcelize
    object Dot : Shape() {
        override fun toString() = "Dot"
    }
}

class Money(val amount: Long, val currency: String) : java.io.Serializable {
    override fun equals(other: Any?) = other is Money && other.amount == amount && other.currency == currency
    override fun hashCode() = amount.hashCode()
    override fun toString() = "$amount $currency"
}

@Parcelize
data class Primitives(
    val s: String, val ns: String?, val i: Int, val ni: Int?, val l: Long, val nl: Long?,
    val b: Boolean, val nb: Boolean?, val d: Double, val nd: Double?, val f: Float, val nf: Float?,
    val c: Char, val nc: Char?, val by: Byte, val nby: Byte?, val sh: Short, val nsh: Short?,
) : Parcelable

@Parcelize
data class Nested(
    val color: Color, val ncolor: Color?, val inner: Inner, val ninner: Inner?, val empty: Empty, val nempty: Empty?,
    val shape: Shape, val nshape: Shape?, val circle: Shape.Circle, val money: Money, val nmoney: Money?,
    val big: BigDecimal, val nbig: BigDecimal?, val point: LibPoint, val npoint: LibPoint?, val open: LibOpen,
    val base: Base, val deep: Outer.Deep,
) : Parcelable {
    @IgnoredOnParcel
    val twice = inner.x * 2
}

@Parcelize
data class Collections(
    val strings: List<String>, val nstrings: List<String>?, val nullableStrings: List<String?>, val arrayList: ArrayList<String>,
    val inners: List<Inner>, val ninners: List<Inner>?, val ints: List<Int?>, val longs: MutableList<Long>,
    val set: Set<String>, val colors: Set<Color>, val map: Map<String, Inner>, val nmap: Map<Int, Long>?,
    val hashMap: HashMap<String, Int>, val nested: List<List<Long>>, val mapOfLists: Map<String, List<Inner?>>,
    val shapes: List<Shape>,
) : Parcelable

class Outer {
    @Parcelize
    data class Deep(val id: Long) : Parcelable
}

// An open class's members aren't final, and other classes write it with
// writeParcelable.
@Parcelize
open class Base(val tag: String) : Parcelable {
    override fun equals(other: Any?) = other is Base && other.tag == tag
    override fun hashCode() = tag.hashCode()
    override fun toString() = "Base($tag)"
}

// A class's own describeContents stays.
@Parcelize
class Described(private val v: Int) : Parcelable {
    override fun describeContents() = 7
    override fun equals(other: Any?) = other is Described && other.v == v
    override fun hashCode() = v
    override fun toString() = "Described($v)"
}

// An enum class writes its name, which createFromParcel looks up. Others
// of the module write it with its writeToParcel, but one whose entries
// have bodies, which isn't final, with writeParcelable.
@Parcelize
enum class Level : Parcelable { LOW, HIGH }

@Parcelize
enum class Tone(val code: Int) : Parcelable {
    SOFT(1),
    LOUD(2) {
        override fun shout() = "!"
    };

    open fun shout() = ""
}

// The direct subclasses of a sealed class or interface annotated
// @Parcelize are Parcelable too, wherever in the package they are.
@Parcelize
sealed class Step : Parcelable {
    data class Go(val steps: Int, val level: Level) : Step()

    data object Stop : Step()

    data class Nest(val inner: Inner, val kind: Kind) : Step() {
        @Parcelize
        sealed class Kind : Parcelable {
            data class Named(val name: String) : Kind()
        }
    }
}

data class Jump(val to: Int) : Step()

@Parcelize
sealed interface Mode : Parcelable

class Edit(val id: String, val tags: Array<String>, val more: Array<String>?) : Mode {
    override fun equals(other: Any?) = other is Edit && other.id == id && other.tags.contentEquals(tags) && other.more.contentEquals(more)
    override fun hashCode() = id.hashCode()
    override fun toString() = "Edit($id, ${tags.toList()}, ${more?.toList()})"
}

@Parcelize
data class Modes(
    val level: Level, val nlevel: Level?, val tone: Tone, val ntone: Tone?, val step: Step, val go: Step.Go,
    val stop: Step.Stop, val mode: Mode, val edit: Edit?,
) : Parcelable
