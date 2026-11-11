package lib

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.BindingContainer
import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesIntoMap
import dev.zacsweers.metro.ContributesIntoSet
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.IntoSet
import dev.zacsweers.metro.Named
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.SingleIn
import dev.zacsweers.metro.StringKey

interface Clock {
    fun now(): Long
}

@Inject
@SingleIn(AppScope::class)
@ContributesBinding(AppScope::class)
class FixedClock : Clock {
    init {
        println("FixedClock created")
    }

    override fun now() = 42L
}

interface Plugin {
    val name: String
}

interface Handler {
    fun handle(): String
}

@Inject
@ContributesIntoSet(AppScope::class)
class LibPlugin : Plugin {
    override val name = "lib"
}

@Inject
@StringKey("lib")
@ContributesIntoMap(AppScope::class)
class LibHandler : Handler {
    override fun handle() = "lib handler"
}

@BindingContainer
@ContributesTo(AppScope::class)
object LibBindings {
    @Provides
    @Named("greeting")
    fun greeting(): String = "hello"

    @Provides
    @IntoSet
    fun extraPlugin(): Plugin = object : Plugin {
        override val name = "lib-extra"
    }
}

@Inject
class Formatter(private val clock: Clock) {
    fun format(s: String) = "[${clock.now()}] $s"
}

@ContributesTo(AppScope::class)
interface LibAccessors {
    val formatter: Formatter
}
