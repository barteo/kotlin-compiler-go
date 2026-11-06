package m

import com.squareup.moshi.JsonClass

// Uses Address: dirty when Address.kt changes.
@JsonClass(generateAdapter = true)
data class User(val name: String, val address: Address)
