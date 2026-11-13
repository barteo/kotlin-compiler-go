package d

import dagger.Binds
import dagger.BindsInstance
import dagger.Component
import dagger.Lazy
import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import javax.inject.Singleton

class Clock @Inject constructor() {
    fun now(): Long = 0
}

@Singleton
class Repo @Inject constructor(private val clock: Clock, @Named("url") val url: String, val port: Int)

interface Store {
    fun get(): String
}

class MemStore @Inject constructor(private val repo: Lazy<Repo>, private val clocks: Provider<Clock>) : Store {
    override fun get() = repo.get().url
}

@Module
object NetModule {
    @Provides
    @Named("url")
    fun url(): String = "http://x"

    @Provides
    @IntoSet
    fun plugin(clock: Clock): String = "p${clock.now()}"
}

@Module
interface Bindings {
    @Binds
    fun store(s: MemStore): Store
}

class Target {
    @Inject
    lateinit var clock: Clock

    @Inject
    @Named("url")
    lateinit var url: String
}

@Singleton
@Component(modules = [NetModule::class, Bindings::class])
interface AppComponent {
    fun store(): Store
    val repo: Repo
    fun plugins(): Set<String>
    fun inject(t: Target)

    @Component.Factory
    interface Factory {
        fun create(@BindsInstance port: Int): AppComponent
    }
}
