package app

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesIntoSet
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.GraphExtension
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.SingleIn
import lib.Plugin

abstract class SessionScope private constructor()

abstract class TaskScope private constructor()

interface Auth {
    fun token(): String
}

@SingleIn(SessionScope::class)
@ContributesBinding(SessionScope::class)
class SessionAuth @Inject constructor(private val userId: Long, private val greeter: Greeter) : Auth {
    init {
        println("SessionAuth created")
    }

    override fun token() = "token-$userId (${greeter.greet()})"
}

@Inject
@ContributesIntoSet(SessionScope::class)
class SessionPlugin : Plugin {
    override val name = "session"
}

@Inject
class Task(private val auth: Auth, private val name: String) {
    fun run() = "$name with ${auth.token()}"
}

@GraphExtension(TaskScope::class)
interface TaskGraph {
    val task: Task

    @Provides
    fun name(): String = "task"
}

@GraphExtension(SessionScope::class)
interface SessionGraph {
    val auth: Auth
    val plugins: Set<Plugin>
    val tasks: TaskGraph
    val newTask: () -> TaskGraph

    @GraphExtension.Factory
    @ContributesTo(AppScope::class)
    fun interface Factory {
        fun createSession(@Provides userId: Long): SessionGraph
    }
}
