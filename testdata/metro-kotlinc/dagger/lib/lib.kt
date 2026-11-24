package lib

import dagger.Binds
import dagger.Lazy
import dagger.Module
import dagger.Provides
import dagger.assisted.Assisted
import dagger.assisted.AssistedFactory
import dagger.assisted.AssistedInject
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.StringKey
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import javax.inject.Singleton

interface Repo { fun name(): String }

@Singleton
class RepoImpl @Inject constructor(@Named("prefix") private val prefix: String, private val n: Provider<Int>, private val l: Lazy<Long>) : Repo {
    override fun name() = prefix + "repo " + n.get() + " " + l.get()
}

class Box<T> @Inject constructor(val value: T)

class Counter @Inject constructor() {
    val id = next++

    companion object {
        var next = 0
    }
}

@dev.zacsweers.metro.HasMemberInjections
open class Base {
    @Inject @Named("prefix") lateinit var basePrefix: String
}

class Members : Base() {
    @Inject lateinit var repo: Repo
    @Inject @JvmField var n: Int = 0
}

class Holder<T : Any> {
    @Inject lateinit var value: T
}

class Greeter @AssistedInject constructor(@Assisted val name: String, @Assisted("times") val times: Int, val repo: Repo) {
    fun greet() = "hi $name from ${repo.name()} x$times"
}

@AssistedFactory
interface GreeterFactory {
    fun create(@Assisted("times") times: Int, name: String): Greeter
}

@Module
object ObjModule {
    @Provides @Named("prefix") fun prefix(): String = "lib "
    @Provides fun num(): Int = 3
    @Provides @IntoSet fun a(): String = "a"
    @Provides @IntoMap @StringKey("k") fun k(): Long = 7L
    @Provides fun lng(): Long = 7L
}

@Module
class ClassModule {
    @Provides @IntoSet fun b(n: Int, counter: Provider<Counter>): String = "b$n" + counter.get().id + counter.get().id
}

@Module
abstract class AbsModule {
    @Binds abstract fun repo(impl: RepoImpl): Repo
}

@Module
@InstallIn(SingletonComponent::class)
object HiltModule {
    @Provides @Named("hilt") fun hilt(repo: Repo): String = "hilt " + repo.name()
}

@EntryPoint
@InstallIn(SingletonComponent::class)
interface HiltEntryPoint {
    @Named("hilt") fun hilt(): String
}
