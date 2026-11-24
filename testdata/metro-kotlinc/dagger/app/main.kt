// metro: interop-include-hilt-annotations=true
package app

import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.createGraph
import javax.inject.Singleton
import lib.*

@Singleton
@DependencyGraph(Singleton::class, bindingContainers = [ObjModule::class, ClassModule::class, AbsModule::class])
interface AppGraph {
    val repo: Repo
    val box: Box<Repo>
    val greeters: GreeterFactory
    fun inject(m: Members)
    fun inject(h: Holder<Int>)
    val set: Set<String>
    val map: Map<String, Long>
}

fun main() {
    val g = createGraph<AppGraph>()
    println(g.repo.name() + " " + (g.repo === g.box.value))
    println(g.greeters.create(2, "x").greet())
    val m = Members()
    g.inject(m)
    println(m.basePrefix + m.repo.name() + " " + m.n)
    val h = Holder<Int>()
    g.inject(h)
    println(h.value)
    println("${g.set.sorted()} ${g.map}")
    println((g as HiltEntryPoint).hilt())
}
