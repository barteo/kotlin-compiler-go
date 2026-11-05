package m

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

// Moshi's KSP code generation over a few common model shapes.

@JsonClass(generateAdapter = true)
data class User(
    val id: Long,
    val name: String,
    @Json(name = "e_mail") val email: String?,
    val tags: List<String> = emptyList(),
    val role: Role = Role.MEMBER,
)

enum class Role { ADMIN, MEMBER }

@JsonClass(generateAdapter = true)
data class Page<T>(val items: List<T>, val next: String?, val total: Int)

@JsonClass(generateAdapter = true)
class Settings(val flags: Map<String, Boolean>) {
    var theme: String = "light"
    @Transient var cache: Any? = null
}

@JsonClass(generateAdapter = true)
data class Nested(val user: User, val pages: List<Page<User>>, val scores: IntArray)
