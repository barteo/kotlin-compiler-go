package k

import me.tatarka.inject.annotations.Assisted
import me.tatarka.inject.annotations.Component
import me.tatarka.inject.annotations.Inject
import me.tatarka.inject.annotations.IntoMap
import me.tatarka.inject.annotations.IntoSet
import me.tatarka.inject.annotations.Provides
import me.tatarka.inject.annotations.Qualifier
import me.tatarka.inject.annotations.Scope

typealias Producer<T> = () -> T

typealias InjectAlias = Inject

typealias Greeting = String

typealias greet = (Int) -> String

@Qualifier
annotation class Named(val value: String)

@Scope
annotation class Session

@InjectAlias
class Clock

@Inject
class Counter(val clock: Clock, val start: Int = 0)

@Inject
data class Request(@Assisted val path: String, @Assisted val retries: Int, val clock: Clock)

@JvmInline
value class Token(val value: String)

@Inject
@Session
class Cache(val token: Token, val service: JavaService)

@Inject
fun greet(clock: Clock, @Assisted times: Int): String = clock.toString().repeat(times)

@Component
abstract class GraphComponent(@get:Provides val token: Token) {
    abstract val counter: Counter
    abstract val requests: (String, Int) -> Request
    abstract val clocks: Producer<Clock>
    abstract val lazyClock: Lazy<Clock>
    abstract val greet: greet
    abstract val named: @Named("a") String
    abstract val handlers: Map<String, Producer<Any?>>
    abstract val tags: Set<Greeting>

    @Provides
    fun greeting(): Greeting = "hi"

    @Provides
    fun service(clock: Clock): JavaService = JavaService(clock)

    @Provides
    fun namedA(): @Named("a") String = "a"

    @Provides
    @IntoMap
    fun handler(f: Producer<Clock>): Pair<String, Producer<Any?>> = "clock" to f

    @Provides
    @IntoSet
    fun tag(): Greeting = "tag"

    companion object
}

@Session
@Component
abstract class SessionComponent(@Component val parent: GraphComponent) {
    abstract val cache: Cache
}

@Component
interface SelfComponent {
    val counter: Counter

    companion object : SelfComponent by SelfComponent::class.create()
}
