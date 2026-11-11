package p

import android.os.Binder
import android.os.Bundle
import android.os.IBinder
import android.os.Parcelable
import android.os.PersistableBundle
import android.util.Size
import android.util.SizeF
import android.util.SparseArray
import android.util.SparseBooleanArray
import android.util.SparseIntArray
import android.util.SparseLongArray
import java.io.FileDescriptor
import kotlinx.parcelize.IgnoredOnParcel
import kotlinx.parcelize.Parcelize
import kotlinx.parcelize.RawValue
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

// The plugin's own ways of writing Android's and Java's types: each with
// a Parcel method of its own, or the size and the elements, and the
// collections it knows by class. Another class, such as StringBuilder or
// LinkedList, is just Serializable, and an object that isn't Parcelable
// is written as 1, so that a null one reads back as the object.
object Marker {
    override fun toString() = "Marker"
}

object Twice : (Int) -> Int, java.io.Serializable {
    override fun invoke(x: Int) = x * 2
    override fun toString() = "Twice"
}

@Parcelize
class Builtins(
    val cs: CharSequence, val ncs: CharSequence?, val bundle: Bundle, val nbundle: Bundle?, val pbundle: PersistableBundle?,
    val ints: IntArray, val nints: IntArray?, val longs: LongArray, val floats: FloatArray, val doubles: DoubleArray,
    val bools: BooleanArray, val bytes: ByteArray, val chars: CharArray, val shorts: ShortArray, val nshorts: ShortArray?,
    val boxed: Array<Long>, val nboxed: Array<Int?>?, val inners: Array<Inner>, val colors: Array<Color?>,
    val binder: IBinder, val nbinder: Binder?, val binders: Array<IBinder>, val binderList: List<IBinder>,
    val size: Size, val nsize: Size?, val sizeF: SizeF, val sba: SparseBooleanArray, val nsba: SparseBooleanArray?,
    val sia: SparseIntArray, val nsia: SparseIntArray?, val sla: SparseLongArray, val sa: SparseArray<String>, val nsa: SparseArray<Inner?>?,
    val hashSet: HashSet<Int>, val deque: ArrayDeque<String>, val jdeque: java.util.ArrayDeque<Long>, val sorted: java.util.SortedSet<String>,
    val nav: java.util.NavigableSet<Int>?, val treeMap: java.util.TreeMap<String, Int>, val sortedMap: java.util.SortedMap<Int, Inner>,
    val conc: java.util.concurrent.ConcurrentHashMap<String, Long>, val marker: Marker, val nmarker: Marker?,
    val builder: StringBuilder, val linked: java.util.LinkedList<Int>, val fn: (Int) -> Int,
) : Parcelable {
    override fun toString() = listOf(
        cs, ncs, bundle, nbundle, pbundle, ints.contentToString(), nints?.contentToString(), longs.contentToString(),
        floats.contentToString(), doubles.contentToString(), bools.contentToString(), bytes.contentToString(),
        chars.contentToString(), shorts.contentToString(), nshorts?.contentToString(), boxed.contentToString(),
        nboxed?.contentToString(), inners.contentToString(), colors.contentToString(), binder, nbinder,
        binders.contentToString(), binderList, size, nsize, sizeF, sba, nsba, sia, nsia, sla, sa, nsa, hashSet, deque,
        jdeque, sorted, nav, treeMap, sortedMap, conc, marker, nmarker, builder, linked, fn(3),
    ).toString()

    override fun equals(other: Any?) = other is Builtins && other.toString() == toString()
    override fun hashCode() = toString().hashCode()
}

// A FileDescriptor makes describeContents 1. (The plugin reads one back as
// a ParcelFileDescriptor, which isn't one, so only nulls round-trip.)
@Parcelize
data class Descriptors(val fd: FileDescriptor?, val fds: List<FileDescriptor>, val nested: Map<String, List<FileDescriptor?>>?) : Parcelable

// A class the plugin has no way to write but writeValue, which @RawValue
// allows, as elements of a container too. It doesn't change how a type
// that the plugin does know is written.
data class Plain(val n: Int)

@Parcelize
data class Raw(
    val p: @RawValue Plain, val np: @RawValue Plain?, val ps: @RawValue List<Plain>, val inner: List<@RawValue Plain?>,
    val any: @RawValue Any?, val m: Map<String, @RawValue Any>, val s: @RawValue String,
) : Parcelable

// A generic class's properties are written as their types erased: each
// type parameter as its bound.
@Parcelize
data class Box<T : Parcelable>(val item: T, val items: List<T>, val maybe: T?) : Parcelable

@Parcelize
class Nums<N : Number, S : java.io.Serializable>(val n: N, val ns: Array<N>?, val s: S?) : Parcelable {
    override fun toString() = "Nums($n, ${ns?.contentToString()}, $s)"
    override fun equals(other: Any?) = other is Nums<*, *> && other.toString() == toString()
    override fun hashCode() = toString().hashCode()
}

@Parcelize
data class RawBox<T>(val v: @RawValue T, val vs: @RawValue List<T>) : Parcelable

@Parcelize
data class Unused<T>(val i: Int) : Parcelable

// A final generic class of the module's is read with its CREATOR.
@Parcelize
data class Boxes(val b: Box<Inner>, val nb: Box<Inner>?, val bs: List<Box<Inner>>, val unused: Unused<String>) : Parcelable
