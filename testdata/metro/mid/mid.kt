package mid

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.BindingContainer
import dev.zacsweers.metro.ClassKey
import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesIntoMap
import dev.zacsweers.metro.ContributesIntoSet
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.SingleIn
import lib.Handler
import lib.Plugin

interface Counter {
    fun next(): Int
}

@Inject
@SingleIn(AppScope::class)
@ContributesBinding(AppScope::class)
class CountingCounter : Counter {
    private var n = 0

    override fun next() = ++n
}

@Inject
@ContributesIntoSet(AppScope::class)
class MidPlugin(private val counter: Counter) : Plugin {
    override val name = "mid"
}

@Inject
@ClassKey
@ContributesIntoMap(AppScope::class)
class MidHandler : Handler {
    override fun handle() = "mid handler"
}

@BindingContainer
@ContributesTo(AppScope::class)
object MidBindings {
    @Provides
    fun ratio(): Double = 1.5
}
