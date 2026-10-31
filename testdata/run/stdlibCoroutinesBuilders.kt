// Builder and lambda parameter inference of kotlinx.coroutines' builders:
// flow { } through the lambda's locals, if and when branches, constructor
// calls, methods whose overloads return one type, null, and destructured
// locals; callbackFlow { } through object expressions' methods; and
// suspendCancellableCoroutine { } through nested lambdas, SAM
// constructors' lambdas, != null checks and extensions of the
// continuation.

import kotlinx.coroutines.*
import kotlinx.coroutines.channels.*
import kotlinx.coroutines.flow.*
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

class Response(val n: Int)
class Detail(val title: String, val n: Int)
enum class Kind { A, B }
sealed interface Mode {
    object Create : Mode
    class Edit(val id: Int) : Mode
}

interface Api {
    suspend fun a(): Response
    suspend fun b(): Response
    suspend fun byId(id: Int): Response
    suspend fun methods(id: String, params: Map<String, String>): Response
    suspend fun pairs(): List<Pair<String, Int>>
}

class Mapper { fun map(r: Response): Int = r.n * 10 }

class Repository(val api: Api, val mapper: Mapper, val pension: Boolean, val mode: Mode) {
    fun mapped() = flow {
        val response = api.a()
        emit(mapper.map(response))
    }

    fun methods(id: String, hide: String?) = flow {
        val response = api.methods(id, params = buildMap { hide?.let { put("hide", it) } })
        emit(response)
    }

    fun byKind(k: Kind) = flow { emit(when (k) { Kind.A -> api.a(); Kind.B -> api.b() }) }

    fun detail() = flow {
        val (title, n) = api.pairs().first()
        emit(Detail(title, n))
    }.catch { throw it }

    fun tile() = flow { emit(if (pension) api.a() to null else api.b() to api.byId(7)) }

    fun optional() = flow {
        try {
            val r = when (val m = mode) {
                Mode.Create -> api.a()
                is Mode.Edit -> api.byId(m.id)
            }
            emit(r)
        } catch (e: ArithmeticException) {
            emit(null)
        }
    }
}

interface Observer { fun onStart(n: Int) }

class Owner {
    val observers = mutableListOf<Observer>()
}

fun interface Listener { fun changed(prefs: Prefs, key: String?) }

class Prefs {
    var listener: Listener? = null
    fun getString(key: String): String? = if (key == "token") "t0k3n" else null
}

class Task(val ok: Boolean) {
    fun onDone(f: (Task) -> Unit) = f(this)
}

class Sources(val owner: Owner, val prefs: Prefs) {
    fun starts() = callbackFlow {
        val observer = object : Observer {
            override fun onStart(n: Int) {
                trySend(n > 0)
            }
        }
        owner.observers += observer
        awaitClose { owner.observers -= observer }
    }

    suspend fun token() = suspendCancellableCoroutine { cont ->
        val listener = Listener { prefs, key ->
            if (cont.isActive) {
                val token = prefs.getString(key ?: "")
                if (token != null) {
                    cont.resumeWith(Result.success(token))
                } else {
                    cont.resumeWith(Result.failure(IllegalStateException("no token")))
                }
            }
        }
        prefs.listener = listener
    }

    fun allowed() = callbackFlow {
        val allowList = listOf("Pin", "Intercom")
        val observer = object : Observer {
            override fun onStart(n: Int) {
                trySend(allowList.none { it.length == n })
            }
        }
        owner.observers += observer
        awaitClose { owner.observers -= observer }
    }

    fun received(): Flow<Int> = callbackFlow {
        val observer = object : Observer {
            override fun onStart(n: Int) {
                trySendBlocking(n * 100)
            }
        }
        owner.observers += observer
        awaitClose { owner.observers -= observer }
    }.buffer(capacity = Channel.UNLIMITED)

    suspend fun update(task: Task) = suspendCancellableCoroutine { continuation ->
        task.onDone { t ->
            if (t.ok) continuation.resume(Unit) else continuation.resumeWithException(IllegalStateException())
        }
    }
}

// transformLatest { } through a destructured parameter and check(x is T).
data class Quad<A, B, C, D>(val a: A, val b: B, val c: C, val d: D)
sealed interface Content {
    class Opened(val ranges: List<Int>) : Content
    object Closed : Content
}
class Summary(val content: Content)

fun summaries(s: Flow<Summary>, index: Flow<Int?>) =
    combine(s, flowOf("breakdown"), index, flowOf(0.5)) { a, b, c, d -> Quad(a, b, c, d) }.transformLatest {
        val (summary, breakdown, selected, ratio) = it
        val content = summary.content
        check(content is Content.Opened, { "not opened" })
        val i = selected ?: content.ranges.lastIndex
        emit(Quad(content, breakdown, i, ratio))
    }.map {
        val (content, breakdown, i, ratio) = it
        "${content.ranges[i]} $breakdown $ratio"
    }

fun main() = runBlocking {
    val api = object : Api {
        override suspend fun a() = Response(1)
        override suspend fun b() = Response(2)
        override suspend fun byId(id: Int) = Response(id)
        override suspend fun methods(id: String, params: Map<String, String>) = Response(params.size)
        override suspend fun pairs() = listOf("first" to 9)
    }
    val repo = Repository(api, Mapper(), false, Mode.Edit(5))
    println(repo.mapped().first())
    println(repo.methods("x", "y").first().n)
    println(repo.byKind(Kind.B).first().n)
    val d = repo.detail().first()
    println("${d.title} ${d.n}")
    println(repo.tile().first().second?.n)
    val o: Response? = repo.optional().first()
    println(o?.n)

    val owner = Owner()
    val prefs = Prefs()
    val sources = Sources(owner, prefs)
    val job = launch { sources.starts().take(2).collect { println("start $it") } }
    yield()
    yield()
    owner.observers.toList().forEach {
        it.onStart(1)
        it.onStart(0)
    }
    job.join()
    val allowed = launch { sources.allowed().take(2).collect { println("allowed $it") } }
    val received = launch { sources.received().take(1).collect { println("received $it") } }
    yield()
    yield()
    owner.observers.toList().forEach { it.onStart(3) }
    owner.observers.toList().forEach { it.onStart(1) }
    allowed.join()
    received.join()
    println(summaries(flowOf(Summary(Content.Opened(listOf(5, 6)))), flowOf(null)).first())
    val token = async { sources.token() }
    yield()
    prefs.listener!!.changed(prefs, "token")
    val t: String = token.await()
    println("token $t")
    sources.update(Task(true))
    println("updated")
}
