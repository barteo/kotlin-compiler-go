package store

import kotlinx.coroutines.*
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.sync.*

class Store(private val name: String) {
    private val mutex = Mutex()
    private val items = ArrayList<String>()

    suspend fun add(item: String) {
        mutex.withLock {
            delay(1)
            items.add(item)
        }
    }

    suspend fun size(): Int = mutex.withLock { items.size }

    fun updates(): Flow<String> = flow {
        for (i in items.indices) {
            emit("$name:${items[i]}")
        }
    }

    suspend fun <R> transaction(block: suspend Store.() -> R): R = mutex.withLock {
        val saved = ArrayList(items)
        try {
            block()
        } catch (e: Exception) {
            items.clear()
            items.addAll(saved)
            throw e
        }
    }

    // Called inside transaction, which holds the lock.
    fun addUnlocked(item: String) {
        items.add(item)
    }

    fun count(): Int = items.size
}

suspend fun loadAll(names: List<String>): List<Int> = coroutineScope {
    names.map { n ->
        async {
            delay(1)
            n.length
        }
    }.awaitAll()
}

fun ticker(n: Int): Flow<Int> = (1..n).asFlow().onEach { delay(1) }
