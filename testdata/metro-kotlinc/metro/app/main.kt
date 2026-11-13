// metro: interop-include-dagger-annotations=true
package app

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.createGraph
import lib.*

@DependencyGraph(AppScope::class, bindingContainers = [Ints::class])
interface AppGraph {
    val svc: Svc
}

fun main() {
    val g = createGraph<AppGraph>()
    println(g.svc.s())
    println((g as SvcAccess).svcAgain().s())
}
