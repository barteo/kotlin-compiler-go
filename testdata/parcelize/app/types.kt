package p

import android.os.Binder
import android.os.Bundle
import android.os.IBinder
import android.os.Parcel
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
import kotlinx.parcelize.Parceler
import kotlinx.parcelize.Parceler as ParcelerAlias
import kotlinx.parcelize.Parcelize
import kotlinx.parcelize.RawValue
import kotlinx.parcelize.TypeParceler
import kotlinx.parcelize.WriteWith
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

// Custom Parcelers: objects that write and read a type their own way.
data class Stamp(val t: Long)

object StampParceler : Parceler<Stamp> {
    override fun create(parcel: Parcel) = Stamp(parcel.readLong())
    override fun Stamp.write(parcel: Parcel, flags: Int) = parcel.writeLong(t)
}

object NullableStampParceler : Parceler<Stamp?> {
    override fun create(parcel: Parcel) = parcel.readLong().let { if (it < 0) null else Stamp(it) }
    override fun Stamp?.write(parcel: Parcel, flags: Int) = parcel.writeLong(this?.t ?: -1)
}

object ShortStampParceler : Parceler<Stamp> {
    override fun create(parcel: Parcel) = Stamp(parcel.readInt().toLong())
    override fun Stamp.write(parcel: Parcel, flags: Int) = parcel.writeInt(t.toInt())
}

object PlusHundred : Parceler<Int> {
    override fun create(parcel: Parcel) = parcel.readInt() - 100
    override fun Int.write(parcel: Parcel, flags: Int) = parcel.writeInt(this + 100)
}

object Upper : Parceler<String> {
    override fun create(parcel: Parcel) = parcel.readString()!!.lowercase()
    override fun String.write(parcel: Parcel, flags: Int) = parcel.writeString(uppercase())
}

object BoolAsByte : Parceler<Boolean> {
    override fun create(parcel: Parcel) = parcel.readByte() != 0.toByte()
    override fun Boolean.write(parcel: Parcel, flags: Int) = parcel.writeByte(if (this) 1 else 0)
}

// A companion object that is a Parceler writes and reads the class, whose
// constructor's parameters needn't be properties.
@Parcelize
data class SelfWritten(val name: String, val n: Int) : Parcelable {
    companion object : Parceler<SelfWritten> {
        override fun create(parcel: Parcel) = SelfWritten(parcel.readString()!!, parcel.readInt())
        override fun SelfWritten.write(parcel: Parcel, flags: Int) {
            parcel.writeString(name)
            parcel.writeInt(n)
        }
    }
}

@Parcelize
class SelfArray(name: String) : Parcelable {
    val name = name
    override fun toString() = "SelfArray($name)"
    override fun equals(other: Any?) = other is SelfArray && other.name == name
    override fun hashCode() = name.hashCode()

    companion object : Parceler<SelfArray> {
        override fun create(parcel: Parcel) = SelfArray(parcel.readString()!!)
        override fun SelfArray.write(parcel: Parcel, flags: Int) = parcel.writeString(name)
        @Suppress("UNCHECKED_CAST")
        override fun newArray(size: Int) = arrayOfNulls<SelfArray>(size + 1) as Array<SelfArray>
    }
}

// A type parameter is written as its erased bound: the class among its
// bounds, Number, or else the first, Comparable<*>.
@Parcelize
data class Cmp<T : Comparable<T>>(val x: @RawValue T, val xs: @RawValue List<T>) : Parcelable

@Parcelize
data class Bounded<T>(val x: T, val y: T?) : Parcelable where T : Comparable<T>, T : Number

// A generic class's Parceler companion object is of the class
// star-projected.
@Parcelize
class GenSelf<T>(val label: String, val extra: @RawValue T?) : Parcelable {
    override fun toString() = "GenSelf($label, $extra)"
    override fun equals(other: Any?) = other is GenSelf<*> && other.label == label
    override fun hashCode() = label.hashCode()

    companion object : Parceler<GenSelf<*>> {
        override fun create(parcel: Parcel): GenSelf<*> = GenSelf(parcel.readString()!!, null)
        override fun GenSelf<*>.write(parcel: Parcel, flags: Int) = parcel.writeString(label)
        @Suppress("UNCHECKED_CAST")
        override fun newArray(size: Int) = arrayOfNulls<GenSelf<*>>(size + 2) as Array<GenSelf<*>>
    }
}

// Parceler companion objects through a type alias and an import alias.
typealias AliasedParceler = Parceler<Aliased>

@Parcelize
class Aliased(n: Int) : Parcelable {
    val m = n * 2
    override fun toString() = "Aliased($m)"
    override fun equals(other: Any?) = other is Aliased && other.m == m
    override fun hashCode() = m

    companion object : AliasedParceler {
        override fun create(parcel: Parcel) = Aliased(parcel.readInt())
        override fun Aliased.write(parcel: Parcel, flags: Int) = parcel.writeInt(m / 2)
    }
}

@Parcelize
class ImportAliased(n: Int) : Parcelable {
    val m = n + 1
    override fun toString() = "ImportAliased($m)"
    override fun equals(other: Any?) = other is ImportAliased && other.m == m
    override fun hashCode() = m

    companion object : ParcelerAlias<ImportAliased> {
        override fun create(parcel: Parcel) = ImportAliased(parcel.readInt())
        override fun ImportAliased.write(parcel: Parcel, flags: Int) = parcel.writeInt(m - 1)
    }
}

// The class's @TypeParceler wins over the property's for the same type; a
// nullable type takes one for the type non-null after a null marker.
@Parcelize
@TypeParceler<Stamp, StampParceler>()
data class Stamps(
    val s: Stamp, val ns: Stamp?, val ss: List<Stamp>,
    @TypeParceler<Stamp, ShortStampParceler>() val o: Stamp,
    @TypeParceler<Stamp?, NullableStampParceler>() val nn: Stamp?,
) : Parcelable

@Parcelize
data class Writes(
    val s: @WriteWith<StampParceler> Stamp, val ns: @WriteWith<StampParceler> Stamp?,
    val nn: @WriteWith<NullableStampParceler> Stamp?, val ss: List<@WriteWith<StampParceler> Stamp>,
) : Parcelable

// A custom Parceler of the elements keeps the plugin from writing an
// array or list with a Parcel method of its own.
@Parcelize
@TypeParceler<Int, PlusHundred>()
@TypeParceler<String, Upper>()
@TypeParceler<Boolean, BoolAsByte>()
class Fast(val ints: IntArray, val strs: List<String>, val arr: Array<String>, val sba: SparseBooleanArray, val n: Int?) : Parcelable {
    override fun toString() = "Fast(${ints.contentToString()}, $strs, ${arr.contentToString()}, $sba, $n)"
    override fun equals(other: Any?) = other is Fast && other.toString() == toString()
    override fun hashCode() = toString().hashCode()
}
