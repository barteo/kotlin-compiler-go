package m

import com.squareup.moshi.JsonClass
import n.*

// Loops over Lots, whose iterator() is an extension of another file: dirty when Address.kt changes.
@JsonClass(generateAdapter = true)
class Batch(val id: Int) {
    @Transient
    var streets = run {
        var s = ""
        for (a in Lots()) s += a.street
        s
    }
}
