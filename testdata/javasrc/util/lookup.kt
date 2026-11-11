package util

// A suspend function's vararg isn't the last parameter of its method,
// which the Continuation is: javac rejects a class that marks it varargs.
class Lookup {
    suspend fun find(id: Int, vararg tags: String): List<String> = tags.map { "$id:$it" }
    fun count(vararg tags: String) = tags.size
}
