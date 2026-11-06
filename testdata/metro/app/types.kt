package app

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.Assisted
import dev.zacsweers.metro.AssistedFactory
import dev.zacsweers.metro.AssistedInject
import dev.zacsweers.metro.BindingContainer
import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesIntoMap
import dev.zacsweers.metro.ContributesIntoSet
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Named
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.Qualifier
import dev.zacsweers.metro.SingleIn
import dev.zacsweers.metro.StringKey
import lib.Clock
import lib.Formatter
import lib.Handler
import lib.Plugin
import mid.Counter

@Qualifier
annotation class Port

interface Repo {
    fun load(): String
}

@Inject
class RealRepo(private val formatter: Formatter, @Named("user") private val user: String) : Repo {
    override fun load() = formatter.format("repo of $user")
}

@Inject
@ContributesIntoSet(AppScope::class)
class AppPlugin : Plugin {
    override val name = "app"
}

@Inject
@StringKey("app")
@ContributesIntoMap(AppScope::class)
class AppHandler(private val repo: Repo) : Handler {
    override fun handle() = "app handler: " + repo.load()
}

interface Greeter {
    fun greet(): String
}

@SingleIn(AppScope::class)
@ContributesBinding(AppScope::class)
class EnglishGreeter(@Named("greeting") private val greeting: String, @Port private val port: Int) : Greeter {
    init {
        println("EnglishGreeter created")
    }

    override fun greet() = "$greeting on port $port"
}

class Expensive @Inject constructor(private val counter: Counter) {
    init {
        println("Expensive created")
    }

    val id = counter.next()
}

@Inject
class WithDefault(private val clock: Clock, private val label: String = "default label") {
    fun describe() = "$label at ${clock.now()}"
}

@AssistedInject
class Download(@Assisted val url: String, @Assisted val retries: Int, private val formatter: Formatter) {
    fun describe() = formatter.format("download $url with $retries retries")
}

@AssistedFactory
fun interface DownloadFactory {
    fun create(url: String, retries: Int): Download
}

class Screen {
    @Inject
    lateinit var formatter: Formatter

    var greeter: Greeter? = null
        @Inject set

    fun show() = formatter.format("screen with " + greeter?.greet())
}

@BindingContainer
class Config(private val port: Int) {
    @Provides
    @Port
    fun port(): Int = port
}

interface Settings {
    val theme: String
}
