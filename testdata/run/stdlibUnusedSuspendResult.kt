// A suspend call whose value a statement drops isn't cast to its type, as
// kotlinc compiles it: a mock that returns Unit for it doesn't fail.
import java.lang.reflect.Proxy
import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

interface Repo {
    suspend fun ids(): LongArray
    suspend fun count(): Int
    suspend fun names(): List<String>
}

class Source(private val repo: Repo, private val two: Int = 1) {
    suspend fun replace(flag: Boolean) {
        if (flag) {
            repo.ids()
            repo.count()
            repo.names()
        }
        repo.ids()
    }

    suspend fun one() {
        repo.names()
    }

    suspend fun two(): Int {
        repo.count()
        if (two > 0) {
            repo.ids()
        }
        return 2
    }

    suspend fun three(): Int = run {
        repo.ids()
        3
    }
}

fun main() {
    val repo = Proxy.newProxyInstance(Repo::class.java.classLoader, arrayOf(Repo::class.java)) { _, _, _ -> Unit } as Repo
    val block: suspend () -> String = {
        val s = Source(repo)
        s.replace(true)
        s.one()
        "done ${s.two()} ${s.three()}"
    }
    block.startCoroutine(Continuation(EmptyCoroutineContext) { println(it.getOrThrow()) })
}
