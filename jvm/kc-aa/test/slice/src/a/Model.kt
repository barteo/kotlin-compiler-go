@file:Suppress("unused")

package a

@Marker
class User<@Marker T>(@Other("x") val id: Int, name: String) {
    @Marker
    fun f(@Marker p: Int) {}

    @get:Marker
    val q: Int get() = 1

    @Marker
    class Nested

    companion object
}
