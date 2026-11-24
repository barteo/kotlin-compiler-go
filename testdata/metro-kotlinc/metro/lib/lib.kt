package lib

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Provider
import dev.zacsweers.metro.Qualifier

@Qualifier
annotation class Greeting

interface Svc { fun s(): String }

@ContributesBinding(AppScope::class)
@Inject
class SvcImpl(val w: Wrappers, @Greeting val greeting: String) : Svc {
    override fun s() = "$greeting svc " + w.sum()
}

@Inject
class Wrappers(val p: Provider<Int>, val f: () -> Int, val l: Lazy<Long>) {
    fun sum() = p() + f() + l.value
}

@ContributesTo(AppScope::class)
interface SvcAccess {
    fun svcAgain(): Svc
}

@dagger.Module
object Ints {
    @dagger.Provides fun i(): Int = 1
    @dagger.Provides fun l(): Long = 2L
    @dagger.Provides @Greeting fun greeting(): String = "hello"
}
