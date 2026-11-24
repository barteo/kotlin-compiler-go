// metro: interop-include-dagger-annotations=true
package app

import dagger.BindsInstance
import dagger.BindsOptionalOf
import dagger.Lazy
import dagger.Module
import dagger.Provides
import dagger.assisted.Assisted
import dagger.assisted.AssistedFactory
import dagger.multibindings.ClassKey
import dagger.multibindings.ElementsIntoSet
import dagger.multibindings.IntoMap
import dagger.multibindings.Multibinds
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.createGraphFactory
import java.util.Optional
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import lib.CompanionModule
import lib.Greeter
import lib.GreeterModule
import lib.Plugin
import lib.Range
import lib.ToolsModule
import kotlin.reflect.KClass

interface Handler
class AHandler @Inject constructor() : Handler
class BHandler @Inject constructor() : Handler

interface Missing

object Calls {
    var n = 0
}

class Expensive @Inject constructor() {
    init {
        Calls.n++
    }
}

@Module
abstract class OptionalModule {
    @BindsOptionalOf abstract fun greeter(): Greeter
    @BindsOptionalOf abstract fun missing(): Missing
    @Multibinds abstract fun none(): Set<Missing>
}

@Module(includes = [OptionalModule::class, GreeterModule::class])
class AppModule {
    @Provides @IntoMap @ClassKey(AHandler::class) fun a(h: AHandler): Handler = h
    @Provides @IntoMap @ClassKey(BHandler::class) fun b(h: BHandler): Handler = h
    @Provides @ElementsIntoSet fun more(): Set<Plugin> = setOf(Plugin("b"), Plugin("c"))
}

@AssistedFactory
interface RangeFactory {
    fun create(@Assisted("b") end: Int, @Assisted("a") start: Int): Range
}

@DependencyGraph(bindingContainers = [AppModule::class, ToolsModule::class, CompanionModule::class])
interface G {
    val greeter: Greeter
    val greeters: Map<String, Greeter>
    val handlers: Map<KClass<*>, Handler>
    val plugins: Set<Plugin>
    val none: Set<Missing>
    val maybeGreeter: Optional<Greeter>
    val maybeMissing: Optional<Missing>
    val lazy: Lazy<Expensive>
    val providers: dagger.internal.Provider<Expensive>
    val greeterProviders: Map<String, Provider<Greeter>>
    val ranges: RangeFactory
    val tool: String
    val count: Int
    @Named("user") val user: String

    @DependencyGraph.Factory
    interface Factory {
        fun create(@BindsInstance @Named("user") user: String): G
    }
}

fun main() {
    val g = createGraphFactory<G.Factory>().create("ann")
    println(g.greeter.greet() + " " + g.greeters.keys)
    println(g.handlers.keys.map { it.java.simpleName }.sorted())
    println(g.plugins.map { it.name }.sorted())
    println(g.none.size)
    println(g.maybeGreeter.map { it.greet() }.orElse("none") + " " + g.maybeMissing.isPresent)
    val l = g.lazy
    println(Calls.n)
    println(l.get() === l.get())
    println(Calls.n)
    val p = g.providers
    println(p.get() === p.get())
    println(Calls.n)
    println(g.greeterProviders.getValue("en").get().greet())
    println(g.ranges.create(9, 1))
    println(g.tool + " " + g.count + " " + g.user)
}
