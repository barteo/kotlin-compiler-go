package app.a

import dev.zacsweers.metro.GraphExtension
import dev.zacsweers.metro.Provides

@GraphExtension
interface Child {
    val letter: Char

    @Provides
    fun letter(): Char = 'a'
}
