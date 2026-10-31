// Overrides that inherit their default values from the function they
// override, as a repository class implementing its interface: callers
// leave the arguments out through the overridden function's $default
// method.
package lib

interface Repo {
    fun get(refresh: Boolean, clear: Boolean = false): String

    suspend fun start(kind: String, ref: String? = null): String
}

class DefaultRepo : Repo {
    override fun get(refresh: Boolean, clear: Boolean) = "get $refresh $clear"

    override suspend fun start(kind: String, ref: String?) = "start $kind $ref"
}

open class Base {
    open fun greet(name: String, punct: String = "!") = "hi $name$punct"
}

open class Middle : Base() {
    override fun greet(name: String, punct: String) = "hey $name$punct"
}

class Leaf : Middle() {
    override fun greet(name: String, punct: String) = "yo $name$punct"
}

interface Store<T> {
    fun put(item: T, times: Int = 1): String
}

class Names : Store<String> {
    override fun put(item: String, times: Int) = item.repeat(times)
}
