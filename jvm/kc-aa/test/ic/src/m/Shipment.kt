package m

import com.squareup.moshi.JsonClass

// Uses Address by a typealias of another file: dirty when Address.kt changes.
@JsonClass(generateAdapter = true)
data class Shipment(val to: n.Dest, val weight: Int)
