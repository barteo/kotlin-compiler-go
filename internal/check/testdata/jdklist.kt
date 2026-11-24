// The deprecations of JDK 21's List.getFirst() and getLast() and JDK
// 11's Collection.toArray(IntFunction) below, as TestWarningsJDKList reads
// them: each line ends with a comment naming the warnings it reports, if
// any, or "error". They follow kotlinc 2.4.21's JvmMappedScope and the
// tests newListMethods.kt and implementationsForSequencedCollection.kt
// of its diagnostics tests, for JDK 21.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_VARIABLE")

import java.util.function.IntFunction

class A<T> : ArrayList<T>() {
    override fun getFirst(): T = super.getFirst() // OVERRIDE_DEPRECATION DEPRECATION
    fun superLast(): T = super.last // DEPRECATION
    override fun <R> toArray(generator: IntFunction<Array<R>>): Array<R> { // OVERRIDE_DEPRECATION
        return super.toArray(generator) // DEPRECATION
    }
}

abstract class B<T> : List<T> {
    override fun getLast(): T = super.getLast() // OVERRIDE_DEPRECATION DEPRECATION
}

fun calls(x: MutableList<String>, y: ArrayList<String>, z: A<String>, ll: java.util.LinkedList<String>, jad: java.util.ArrayDeque<String>) {
    x.getFirst() // error
    x.first()
    y.getFirst() // DEPRECATION
    y.first // DEPRECATION
    y.first()
    z.getFirst() // DEPRECATION
    z.first // DEPRECATION
    ll.getLast() // DEPRECATION
    ll.last // DEPRECATION
    jad.getFirst()
    jad.first
    y.addFirst("")
    y.removeLast()
}
