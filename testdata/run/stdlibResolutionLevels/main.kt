// Each level of scope gives way to the next when none of its functions
// apply, and @LowPriorityInOverloadResolution and Java's primitive
// overloads decide between candidates that do.

interface Handle { fun dispose() }

interface Job {
    fun onDone(handler: (Throwable?) -> Unit): Handle
    fun onDone(cancelling: Boolean, immediately: Boolean, handler: (Throwable?) -> Unit): Handle
}

abstract class Node { abstract fun invoke(cause: Throwable?) }
class Resume(val name: String) : Node() { override fun invoke(cause: Throwable?) = println("node $name $cause") }

// Like kotlinx.coroutines' internal Job.invokeOnCompletion(handler: JobNode).
fun Job.onDone(immediately: Boolean = true, handler: Node): Handle = onDone(false, immediately, handler::invoke)

class JobImpl : Job {
    override fun onDone(handler: (Throwable?) -> Unit): Handle = onDone(false, true, handler)
    override fun onDone(cancelling: Boolean, immediately: Boolean, handler: (Throwable?) -> Unit): Handle {
        handler(null)
        return object : Handle { override fun dispose() = println("disposed $cancelling $immediately") }
    }

    // The member onDone(handler) doesn't take a Node: the extension does.
    fun join() = onDone(handler = Resume("join"))
}

interface Cont {
    fun onCancel(handler: (Throwable?) -> Unit)
    fun onCancel(segment: String, index: Int)
}

interface Cancel {
    fun invoke(cause: Throwable?)
    class User(val handler: (Throwable?) -> Unit) : Cancel { override fun invoke(cause: Throwable?) = handler(cause) }
}

fun Cont.onCancel(handler: Cancel) = when (this) {
    is ContImpl -> handler.invoke(null)
    else -> println("other")
}

class ContImpl : Cont {
    override fun onCancel(segment: String, index: Int) = println("$segment $index")

    // Its own return type isn't needed: the call is the extension's.
    override fun onCancel(handler: (Throwable?) -> Unit) =
        onCancel(Cancel.User(handler))
}

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
@kotlin.internal.LowPriorityInOverloadResolution
fun <T> block(context: String = "low", body: () -> T): T = block(context, body)

@JvmName("blockK")
fun <T> block(context: String = "high", body: () -> T): T {
    println(context)
    return body()
}

fun main() {
    JobImpl().join().dispose()
    ContImpl().onCancel { println("cancel $it") }
    println(block { 1 })
    println(block("ctx") { 2 })
    // getLong(String, long), not getLong(String, Long).
    println(java.lang.Long.getLong("no.such.property", 42L))
    time.run()
}
