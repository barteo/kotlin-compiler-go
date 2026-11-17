package k

import me.tatarka.inject.annotations.Assisted
import me.tatarka.inject.annotations.Component
import me.tatarka.inject.annotations.Inject
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.AppScope
import software.amazon.lastmile.kotlin.inject.anvil.ContributesBinding
import software.amazon.lastmile.kotlin.inject.anvil.ContributesSubcomponent
import software.amazon.lastmile.kotlin.inject.anvil.ContributesTo
import software.amazon.lastmile.kotlin.inject.anvil.ForScope
import software.amazon.lastmile.kotlin.inject.anvil.MergeComponent
import software.amazon.lastmile.kotlin.inject.anvil.SingleIn

interface Authenticator {
    fun id(): String
}

interface Interceptor

interface Base

interface Other

interface Loader {
    fun load(): String?
}

interface Store<in K : Any, out V> {
    fun get(key: K): V?
}

@Inject
@SingleIn(AppScope::class)
@ContributesBinding(AppScope::class)
class RealAuthenticator : Authenticator {
    override fun id(): String = "real"
}

@Inject
@ContributesBinding(AppScope::class, multibinding = true)
class LoggingInterceptor : Interceptor

@Inject
@ContributesBinding(scope = AppScope::class, multibinding = true)
class RetryInterceptor(@ForScope(AppScope::class) val retries: Int) : Interceptor

@Inject
@ContributesBinding(AppScope::class, boundType = Base::class)
@ContributesBinding(AppScope::class, boundType = Other::class, multibinding = true)
class Impl : Base, Other

@Inject
@ContributesBinding(AppScope::class)
class StringLoader(@Assisted val path: String) : Loader {
    override fun load(): String? = path
}

@Inject
class Screen(
    val loaderFactory: (String) -> Loader,
    val store: Store<String, Number>,
    val lazyAuth: Lazy<Authenticator>,
    val interceptors: Set<Interceptor>,
)

@Inject
class NumberStore : Store<String, Number> {
    override fun get(key: String): Number? = key.length
}

@ContributesTo(AppScope::class)
interface AppIdComponent {
    @Provides
    fun appId(): String = "app"

    @Provides
    @ForScope(AppScope::class)
    fun retries(): Int = 3
}

@ContributesTo(AppScope::class)
interface StoreComponent {
    @Provides
    fun store(impl: NumberStore): Store<String, Number> = impl
}

@ContributesTo(AppScope::class)
interface ExcludedComponent {
    @Provides
    fun excluded(): Long = 1L
}

abstract class LoggedInScope private constructor()

@Inject
@SingleIn(LoggedInScope::class)
class Renderer(val user: UserId, val auth: Authenticator)

@JvmInline
value class UserId(val value: String)

@ContributesSubcomponent(LoggedInScope::class)
@SingleIn(LoggedInScope::class)
interface RendererComponent {
    val renderer: Renderer

    @ContributesSubcomponent.Factory(AppScope::class)
    interface Factory {
        fun createRendererComponent(user: UserId): RendererComponent
    }
}

class Config(val name: String = "default")

@MergeComponent(AppScope::class, exclude = [ExcludedComponent::class])
@SingleIn(AppScope::class)
abstract class AppComponent(
    @get:Provides val config: Config,
) {
    abstract val auth: Authenticator
    abstract val interceptors: Set<Interceptor>
    abstract val screen: Screen
    abstract val base: Base
    abstract val others: Set<Other>
    abstract val fake: Fake

    companion object
}

@Component
@MergeComponent(AppScope::class)
@SingleIn(AppScope::class)
abstract class PlainComponent(
    @get:Provides val config: Config = Config(),
) : PlainComponentMerged {
    abstract val auth: Authenticator
    abstract val renderers: RendererComponent.Factory
}

interface Fake

@Inject
@ContributesBinding(AppScope::class, boundType = Fake::class)
object FakeObject : Fake

@Inject
@ContributesBinding(AppScope::class, replaces = [RealAuthenticator::class])
class FakeAuthenticator : Authenticator {
    override fun id(): String = "fake"
}

