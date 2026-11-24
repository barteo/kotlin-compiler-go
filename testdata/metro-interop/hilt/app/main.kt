// metro: interop-include-hilt-annotations=true enable-kclass-to-class-interop=true optional-binding-behavior=DISABLED
package app

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.createGraph
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Singleton
import lib.Counter
import lib.Feature
import lib.FeatureScoped
import lib.LibEntryPoint
import lib.Marker
import lib.MarkerEntryPoint

interface Repo {
    fun name(): String
}

class RepoImpl @Inject constructor(private val counter: Counter) : Repo {
    override fun name() = "repo ${counter.n}"
}

class Service @Inject constructor(val greeting: String, val repo: Repo, @Named("nested") val nested: String)

@Module
@InstallIn(SingletonComponent::class)
interface AppModule {
    @Binds
    fun repo(impl: RepoImpl): Repo
}

@EntryPoint
@InstallIn(SingletonComponent::class)
interface AppEntryPoint {
    fun service(): Service
}

object Outer {
    @Module
    @InstallIn(SingletonComponent::class)
    object NestedModule {
        @Provides
        @Named("nested")
        fun nested(): String = "nested"
    }

    @EntryPoint
    @InstallIn(SingletonComponent::class)
    interface NestedEntryPoint {
        @Named("nested")
        fun nested(): String
    }
}

@Singleton
@DependencyGraph(Singleton::class)
interface AppGraph {
    val repo: Repo
}

@FeatureScoped
@DependencyGraph(FeatureScoped::class)
interface FeatureGraph {
    val feature: Feature
}

fun main() {
    val g = createGraph<AppGraph>()
    val lib = g as LibEntryPoint
    lib.counter().n++
    println("${lib.greeting()} ${lib.counter() === lib.counter()} ${g.repo.name()}")
    val s = (g as AppEntryPoint).service()
    println("${s.greeting} ${s.repo.name()} ${s.nested}")
    println((g as Outer.NestedEntryPoint).nested())
    println((g as MarkerEntryPoint).marker().name)
    val f = createGraph<FeatureGraph>()
    println("${f.feature.name} ${f.feature === f.feature}")
}
