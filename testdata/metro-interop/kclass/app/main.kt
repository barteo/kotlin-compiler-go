// metro: interop-include-dagger-annotations=true enable-kclass-to-class-interop=true
package app

import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.createGraph
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import kotlin.reflect.KClass
import lib.DaggerScreens
import lib.Declared
import lib.MetroScreens
import lib.Screen

class Router @Inject constructor(val screens: Map<Class<*>, Screen>)

@DependencyGraph(bindingContainers = [DaggerScreens::class, Declared::class, MetroScreens::class])
interface G {
    val classes: Map<Class<*>, Screen>
    val kclasses: Map<KClass<*>, Screen>
    val function: () -> Map<Class<*>, Screen>
    val lazy: Lazy<Map<Class<*>, Screen>>
    val provider: Provider<Map<Class<*>, Screen>>
    val providers: Map<Class<*>, () -> Screen>
    val router: Router
    @Named("none") val none: Map<Class<*>, Screen>
    @Named("none") val noneK: Map<KClass<*>, Screen>
}

fun names(m: Map<Class<*>, *>) = m.keys.map { it.simpleName }.sorted()

fun main() {
    val g = createGraph<G>()
    println(names(g.classes).toString() + " " + g.classes.keys.all { it is Class<*> })
    println(g.kclasses.keys.map { it.java.simpleName }.sorted())
    println(g.classes[lib.Home::class.java]!!::class.java.simpleName + " " + g.kclasses[lib.About::class]!!::class.java.simpleName)
    println(names(g.function()))
    println(names(g.lazy.value))
    println(names(g.provider.get()))
    println(g.providers.getValue(lib.Settings::class.java)()::class.java.simpleName)
    println(names(g.router.screens))
    println(g.none.size.toString() + " " + g.noneK.size)
}
