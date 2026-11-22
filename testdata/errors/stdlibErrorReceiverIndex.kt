// A call on a receiver whose type is an error is a call of the only
// extension of its name that may take the arguments, as in kotlinc: m[1]
// is the standard library's Map<out K, V>.get(key: K), whose V only the
// receiver would give a type, and not MatchGroupCollection.get(name:
// String), which takes no Int.
fun take(i: Int) {}

fun indexed(k: Int) {
    val m = Unknown234(1) // ERROR unresolved reference: Unknown234
    m[1] // ERROR cannot infer type for type parameter 'V'
    m.get(1) // ERROR cannot infer type for type parameter 'V'
    m?.get(1) // ERROR cannot infer type for type parameter 'V'
    val i = m[k] // ERROR cannot infer type for type parameter 'V'
    take(i)
    // Both take a String, and none two arguments.
    m["a"]
    m[1, 2]
    // An argument whose type is an error gives its type parameter that.
    m[i]
    m[i] = 1
    m[1] = i
    m[1] = 2
}
