// A Java List's remove(int) is removeAt, also one it inherits from a
// class that isn't a list; a List<? super T> is a MutableList<in T>,
// which no read-only list is. remove(1) is the standard library's
// deprecated MutableList<T>.remove(index), as in kotlinc.
fun main(a: A, ml: MutableList<Any>, l: List<Any>) {
    a.remove("")
    a.remove(1) // ERROR is deprecated
    a.removeAt(1)
    a.bar(ml)
    a.bar(l) // ERROR type mismatch
}
