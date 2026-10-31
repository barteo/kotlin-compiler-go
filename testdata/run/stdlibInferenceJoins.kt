// Inference that joins what several places say about a type: builder
// lambdas' calls, the branches of an if whose value a lambda drops, and
// the common supertypes of classes that also implement Java interfaces.
import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.resume
import kotlin.coroutines.startCoroutine
import kotlin.coroutines.suspendCoroutine

typealias Listener = java.util.EventListener

sealed class Edu(val key: String) {
    data class What(val t: String) : Listener, Edu("what")
    object Risk : Edu("risk")
    object Acc : Edu("acc")
}

fun mapEdu(e: Edu) = e.key

sealed class Note {
    data class Header(val t: String) : Note()
    data class Body(val n: Int) : Note()
}

class Mapper {
    // add's Header and addAll's Body make a List<Note>.
    fun notes(items: List<Int>) = buildList {
        add(Note.Header("h"))
        addAll(items.map(::body))
    }

    fun lambdaNotes(items: List<Int>) = buildList {
        add(Note.Header("h"))
        addAll(items.map { body(it) }.filter { it.n > 0 })
    }

    private fun body(n: Int) = Note.Body(n)
}

// A builder's receiver passed to a member of a value of a known type.
class Source<T>(private val xs: List<T>) {
    fun <R> map(f: (T) -> R) = Source(xs.map(f))
    fun into(out: MutableList<T>) {
        out.addAll(xs)
    }
}

sealed class Res<T> {
    class Loading<T> : Res<T>() {
        override fun toString() = "Loading"
    }

    data class Done<T>(val value: T) : Res<T>()
}

class UseCase<T>(private val xs: List<T>) {
    fun run() = buildList {
        add(Res.Loading())
        Source(xs).map { done(it) }.into(this)
    }

    private fun done(x: T): Res<T> = Res.Done(x)
}

// resume(Unit) and resume(null) make a Unit?.
suspend fun resumes(ok: Boolean) = suspendCoroutine { c ->
    if (ok) c.resume(Unit) else c.resume(null)
}

class Scope {
    val tags = mutableListOf<String>()
}

fun item(tags: (Scope.() -> Unit)? = null): List<String> {
    val s = Scope()
    tags?.invoke(s)
    return s.tags
}

// Variant's member on an E bounded by Enum and Variant, read through a
// star projection.
interface Variant {
    val serializedName: String
}

enum class Color(override val serializedName: String) : Variant { RED("r"), GREEN("g") }

class Experiment<E>(val variants: List<E>) where E : Enum<E>, E : Variant

fun <E> describe(e: E) where E : Enum<E>, E : Variant = e.serializedName + e.name

fun names(x: Experiment<*>) = x.variants.map { it.serializedName }

fun main() {
    println(listOf(Edu.What("t"), Edu.Risk, Edu.Acc).asSequence().map { mapEdu(it) }.toList())
    println(Mapper().notes(listOf(1, 2)))
    println(Mapper().lambdaNotes(listOf(0, 3)))
    println(UseCase(listOf("a", "b")).run())
    suspend {
        println(resumes(true))
        println(resumes(false))
    }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    for (selected in listOf(true, false)) {
        println(item(tags = {
            if (selected) {
                listOf("a")
            } else {
                emptyList()
            }
        }))
    }
    var n = 0
    val f: () -> Unit = { if (n > 0) n = 1 else n = 2 }
    f()
    println(n)
    val t = Throwable(message = "Oh my")
    println(t.message + Throwable(cause = t).cause?.message + Throwable(message = "a", cause = null).message)
    println(describe(Color.RED))
    println(names(Experiment(Color.entries)))
}
