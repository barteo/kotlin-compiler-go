// metro: interop-include-javax-annotations=true interop-include-jakarta-annotations=true
package app

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.SingleIn
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.createGraph
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import javax.inject.Singleton
import lib.Clock
import lib.Region
import lib.Remote

interface Api { val name: String }
class RealApi @Inject constructor() : Api { override val name = "real" }

@Singleton
class Counter @Inject constructor() {
    var n = 0
}

class Repo @Inject constructor(val api: Api, @Named("url") val url: String, val counter: Provider<Counter>)

class JakartaUser @jakarta.inject.Inject constructor(val api: jakarta.inject.Provider<Api>, @jakarta.inject.Named("url") val url: String)

class Late {
    @Inject lateinit var repo: Repo
    var viaMethod = ""
    @Inject fun set(@Named("url") url: String) { viaMethod = url }
}

class Defaulted @Inject constructor(val api: Api, val label: String = "default")

@Singleton
@SingleIn(AppScope::class)
@DependencyGraph(AppScope::class)
interface G {
    val remote: Remote
    val clock: Clock
    val repo: Repo
    val counter: Counter
    val jakarta: JakartaUser
    val defaulted: Defaulted
    val counters: Provider<Counter>
    fun inject(l: Late)

    @Provides fun api(r: RealApi): Api = r
    @Provides @Named("url") fun url(): String = "http://x"
    @Provides @jakarta.inject.Named("url") fun jurl(): String = "jakarta://x"
    @Provides @Region fun region(): String = "eu"
}

fun main() {
    val g = createGraph<G>()
    val r = g.repo
    println(r.api.name + " " + r.url)
    r.counter.get().n++
    println(g.counter.n)
    println(g.counters.get() === g.counter)
    println(g.jakarta.api.get().name + " " + g.jakarta.url)
    println(g.defaulted.label)
    val l = Late()
    g.inject(l)
    println(l.repo.url + " " + l.viaMethod)
    val remote = g.remote
    println(remote.url + " " + remote.region + " " + remote.clock.get().now() + " " + g.clock.now())
}
