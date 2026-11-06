package m

import com.squareup.moshi.JsonClass

// Uses neither: clean when Address.kt changes.
@JsonClass(generateAdapter = true)
class Settings(val theme: String) {
    var street: String = ""
}
