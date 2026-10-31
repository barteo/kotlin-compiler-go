import android.os.Parcel
import android.os.Parcelable
import kotlinx.parcelize.Parcelize
import p.*

// A Parcelable that kotlinc compiles against kc's classes: it writes the
// final ones with their writeToParcel and reads them with their CREATOR.
@Parcelize
data class Holder(val inner: Inner, val empty: Empty, val deep: Outer.Deep, val shape: Shape, val base: Base, val color: Color, val level: Level, val step: Step) : Parcelable

fun main() {
    val x = Holder(Inner(7, "seven"), Empty, Outer.Deep(8), Shape.Circle(9.0), Base("ten"), Color.RED, Level.HIGH, Step.Go(1, Level.LOW))
    val parcel = Parcel()
    parcel.writeParcelable(x, 0)
    println("use: ${parcel.log()}")
    val back = parcel.readParcelable<Holder>(Holder::class.java.classLoader)
    println("  read: $back")
    println("  equal: ${back == x}")
}
