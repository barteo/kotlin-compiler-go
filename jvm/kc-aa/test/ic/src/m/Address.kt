package m

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Address(val street: String)
