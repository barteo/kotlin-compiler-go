// kotlinc: -jvm-default=enable
package lib

annotation class Marked(val name: String = "")

@Retention(AnnotationRetention.BINARY)
annotation class Kept

annotation class Arg

class Holder

interface WithDefaults {
    @get:Marked("holder")
    @get:Kept
    val holder: Holder
        get() = Holder()

    @Marked
    @Deprecated("old")
    fun twice(@Arg n: Int): Int = n * 2

    @Marked("extension")
    fun String.tagged(@Arg times: Int): String = "<$this>".repeat(times)

    @get:Marked("half")
    val Int.half: Int
        get() = this / 2
}
