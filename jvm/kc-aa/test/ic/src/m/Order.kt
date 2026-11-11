package m

import com.squareup.moshi.JsonClass

// Uses User, not Address.
@JsonClass(generateAdapter = true)
data class Order(val user: User, val total: Int)
