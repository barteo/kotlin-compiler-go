package lib

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.ContributesBinding
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import javax.inject.Qualifier

@Qualifier
annotation class Region

interface Clock {
    fun now(): String
}

// A class of the class path whose constructor is injected with javax's
// annotations, with qualifiers on its parameters.
class Remote @Inject constructor(@Named("url") val url: String, @Region val region: String, val clock: Provider<Clock>)

@ContributesBinding(AppScope::class)
class FixedClock @Inject constructor() : Clock {
    override fun now() = "noon"
}
