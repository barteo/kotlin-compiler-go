package lib

import android.os.Parcelable
import kotlinx.parcelize.Parcelize

// A final Parcelable of another module, which the plugin writes with its
// writeToParcel and reads with its CREATOR.
@Parcelize
data class LibPoint(val x: Int, val y: Int) : Parcelable

// An open one, which it writes with writeParcelable.
@Parcelize
open class LibOpen(val label: String) : Parcelable {
    override fun equals(other: Any?) = other is LibOpen && other.label == label
    override fun hashCode() = label.hashCode()
    override fun toString() = "LibOpen($label)"
}
