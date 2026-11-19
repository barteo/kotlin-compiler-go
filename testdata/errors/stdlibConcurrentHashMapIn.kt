// in on a ConcurrentHashMap calls its contains(Object), which is
// containsValue: kotlinc rejects it.
class A : java.util.concurrent.ConcurrentHashMap<String, Int>() {
    operator fun contains(x: Char): Boolean = true
}

fun f(hm: java.util.concurrent.ConcurrentHashMap<String, Int>, a: A) {
    "" in hm // ERROR method 'contains' from ConcurrentHashMap
    1 !in a // ERROR method 'contains' from ConcurrentHashMap
    ' ' in a
    hm.contains("")
    "" in (hm as Map<String, Int>)
}
