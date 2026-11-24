// kotlinc: -jvm-default=enable
// Generic signatures of the methods that take the instance first: the
// DefaultImpls methods of an interface's own bodies and of those it
// inherits, with the continuation of a suspend member, and the static
// method that holds the body of an overridable suspend member.
class Event

interface Repo {
    suspend fun confirm(e: Event) {}
    suspend fun count(es: List<Event>): Int = es.size
}

interface SubRepo : Repo

interface Typed<T> {
    suspend fun first(t: T, rest: List<T>): T = t
}

interface SubTyped<Q> : Typed<Q>, Repo

open class Service<T> {
    open suspend fun load(keys: List<T>): Map<T, Event> = emptyMap()
}

fun main() {
    for (c in listOf("Repo", "Repo\$DefaultImpls", "SubRepo\$DefaultImpls", "Typed", "Typed\$DefaultImpls", "SubTyped\$DefaultImpls", "Service")) {
        val ms = Class.forName(c).declaredMethods.filter { !it.name.startsWith("access\$") }
        for (m in ms.sortedBy { it.name }) {
            println("$c.${m.name}: ${m.toGenericString()}")
        }
    }
}
