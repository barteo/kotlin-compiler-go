// lateinit properties and locals: reads before the first assignment
// throw, and isInitialized tells whether they have been assigned.

class Service {
    lateinit var name: String
    private lateinit var secret: String
    protected lateinit var guarded: String
    internal lateinit var shared: List<Int>

    fun describe() = if (this::name.isInitialized) "named " + name else "unnamed"
    fun hide(s: String) {
        secret = s
    }
    fun reveal(): String = secret
    fun ready() = ::secret.isInitialized
    fun later(): Runnable = Runnable { println("later " + secret.length) }
}

lateinit var config: String

fun configured() = ::config.isInitialized

fun attempt(f: () -> Any) {
    try {
        println(f())
    } catch (e: UninitializedPropertyAccessException) {
        println(e.message)
    }
}

fun main() {
    val s = Service()
    println(s.describe())
    attempt { s.name }
    s.name = "svc"
    println(s.describe())
    println(s.name.length)
    attempt { s.reveal() }
    println(s.ready())
    s.hide("abc")
    println(s.reveal() + " " + s.ready())
    s.later().run()
    println(configured())
    attempt { config }
    config = "on"
    println(config + " " + configured())
    lateinit var local: String
    attempt { local }
    local = "set"
    println(local)
    val fields = Service::class.java.declaredFields.map { it.name + ":" + java.lang.reflect.Modifier.toString(it.modifiers) }.sorted()
    println(fields)
}
