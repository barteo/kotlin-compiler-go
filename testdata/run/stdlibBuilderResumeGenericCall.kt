import kotlin.coroutines.resume
import kotlin.coroutines.suspendCoroutine

// T of Result.failure(e), in a lambda of a call in suspendCoroutine's
// block, is the Unit of the Result<Unit> that the other lambda resumes
// the continuation with.
fun subscribe(fail: Boolean, onDone: () -> Unit, onError: (Throwable) -> Unit) {
    if (fail) onError(IllegalStateException("failed")) else onDone()
}

private suspend fun run(fail: Boolean) =
    suspendCoroutine { continuation ->
        subscribe(
            fail,
            { continuation.resume(Result.success(Unit)) },
            { e: Throwable -> continuation.resume(Result.failure(e)) },
        )
    }

suspend fun main() {
    val done: Result<Unit> = run(false)
    println(done)
    println(run(true).exceptionOrNull()?.message)
}
