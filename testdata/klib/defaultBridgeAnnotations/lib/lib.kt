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
}
