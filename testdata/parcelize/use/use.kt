import android.os.Parcel
import android.os.Parcelable
import kotlinx.parcelize.Parcelize
import kotlinx.parcelize.TypeParceler
import kotlinx.parcelize.WriteWith
import p.*

// A Parcelable that kotlinc compiles against kc's classes: it writes the
// final ones with their writeToParcel and reads them with their CREATOR.
@Parcelize
data class Holder(val inner: Inner, val empty: Empty, val deep: Outer.Deep, val shape: Shape, val base: Base, val color: Color, val level: Level, val step: Step, val builtins: Builtins?, val descriptors: Descriptors, val raw: Raw, val box: Box<Inner>, val boxes: Boxes?, val self: SelfWritten, val stamps: Stamps, val mine: UseStamps, val gen: GenSelf<String>, val aliased: Aliased, val bounded: Bounded<Long>, val specials: Specials) : Parcelable

// kc's Parcelers, in a class that kotlinc compiles.
@Parcelize
@TypeParceler<Stamp, StampParceler>()
data class UseStamps(val s: Stamp, val w: @WriteWith<Upper> String) : Parcelable

fun main() {
    val x = Holder(Inner(7, "seven"), Empty, Outer.Deep(8), Shape.Circle(9.0), Base("ten"), Color.RED, Level.HIGH, Step.Go(1, Level.LOW), builtins(nulls = true), Descriptors(null, listOf(), null), raw(), Box(Inner(1, "one"), listOf(), Inner(2, "two")), null, SelfWritten("you", 4), stamps(), UseStamps(Stamp(6), "w"), GenSelf("g", "x"), Aliased(5), Bounded(6L, null), specials())
    val parcel = Parcel()
    parcel.writeParcelable(x, 0)
    println("use: ${parcel.log()}")
    val back = parcel.readParcelable<Holder>(Holder::class.java.classLoader)
    println("  read: $back")
    println("  equal: ${back == x}")
}
