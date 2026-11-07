package h

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.hilt.DefineComponent
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import dagger.multibindings.IntoSet
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Scope
import javax.inject.Singleton

interface Store {
    fun get(): String
}

@Singleton
class MemStore @Inject constructor(@Named("url") private val url: String) : Store {
    override fun get() = url
}

@Module
@InstallIn(SingletonComponent::class)
object NetModule {
    @Provides
    @Named("url")
    fun url(): String = "http://x"

    @Provides
    @IntoSet
    fun plugin(): String = "p"
}

@Module
@InstallIn(SingletonComponent::class)
interface StoreModule {
    @Binds
    fun store(s: MemStore): Store
}

@EntryPoint
@InstallIn(SingletonComponent::class)
interface AppEntryPoint {
    fun store(): Store

    fun plugins(): Set<String>
}

@Scope
@Retention(AnnotationRetention.RUNTIME)
annotation class SessionScoped

@SessionScoped
@DefineComponent(parent = SingletonComponent::class)
interface SessionComponent

@DefineComponent.Builder
interface SessionComponentBuilder {
    fun build(): SessionComponent
}

class Session @Inject constructor(val store: Store)

@Module
@InstallIn(SessionComponent::class)
object SessionModule {
    @Provides
    @SessionScoped
    fun id(): Long = 1
}

@EntryPoint
@InstallIn(SessionComponent::class)
interface SessionEntryPoint {
    fun session(): Session

    fun id(): Long
}
