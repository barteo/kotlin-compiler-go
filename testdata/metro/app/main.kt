package app

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.Binds
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.Includes
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Multibinds
import dev.zacsweers.metro.Named
import dev.zacsweers.metro.Provider
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.createGraph
import dev.zacsweers.metro.createGraphFactory
import kotlin.reflect.KClass
import lib.Formatter
import lib.Handler
import lib.LibAccessors
import lib.Plugin
import mid.Counter

@DependencyGraph(AppScope::class)
interface AppGraph {
    val greeter: Greeter
    val greeterProvider: () -> Greeter
    val counter: Counter
    val counterProvider: Provider<Counter>
    val expensive: Lazy<Expensive>
    val plugins: Set<Plugin>
    val handlers: Map<String, Handler>
    val handlerProviders: Map<String, () -> Handler>
    val classHandlers: Map<KClass<*>, Handler>
    val repo: Repo
    val withDefault: WithDefault
    val downloads: DownloadFactory
    val ratio: Double
    val theme: String

    @Multibinds(allowEmpty = true)
    val empty: Set<CharSequence>

    @Binds
    val RealRepo.bind: Repo

    fun inject(screen: Screen)

    val sessions: SessionGraph.Factory

    @DependencyGraph.Factory
    fun interface Factory {
        fun create(
            @Provides @Named("user") user: String,
            @Includes config: Config,
            @Includes settings: Settings,
        ): AppGraph
    }
}

@DependencyGraph
interface SmallGraph {
    val message: Message

    @Provides
    fun count(): Int = 3
}

@Inject
class Message(private val count: Int) {
    override fun toString() = "message x$count"
}

fun main() {
    val settings = object : Settings {
        override val theme = "dark"
    }
    val graph = createGraphFactory<AppGraph.Factory>().create("ada", Config(8080), settings)
    println("graph created")
    println(graph.greeter.greet())
    println(graph.greeter === graph.greeterProvider())
    println(graph.counter.next())
    println(graph.counterProvider().next())
    println(graph.counter === graph.counterProvider())
    val lazy = graph.expensive
    println("lazy not yet read")
    println(lazy.value.id)
    println(lazy.value === lazy.value)
    println(graph.plugins.map { it.name }.sorted())
    println(graph.handlers.keys.sorted())
    println(graph.handlers.getValue("app").handle())
    println(graph.handlerProviders.getValue("lib")().handle())
    println(graph.classHandlers.keys.map { it.simpleName })
    println(graph.repo.load())
    println(graph.withDefault.describe())
    println(graph.downloads.create("a.zip", 3).describe())
    println(graph.ratio)
    println(graph.theme)
    println(graph.empty.size)
    println((graph as LibAccessors).formatter.format("contributed accessor"))
    val screen = Screen()
    graph.inject(screen)
    println(screen.show())
    println(createGraph<SmallGraph>().message)
    val session = (graph as SessionGraph.Factory).createSession(7)
    println(session.auth.token())
    println(session.auth === session.auth)
    println(session.plugins.map { it.name }.sorted())
    println(session.tasks.task.run())
    println(session.newTask().task.run())
    println(graph.sessions.createSession(8).auth.token())
}
