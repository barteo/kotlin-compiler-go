// A Java class annotated @PurelyImplements is a subtype of the Kotlin
// collection type it names with its type arguments as they are, and the
// members it overrides, its own or inherited, take them as they are too.
// So do the JDK collections K2 knows are pure. Results stay platform
// types, and so do the members only Java declares.
fun pure(s: String?) {
    val a = A<String>()
    a.add(null) // ERROR type mismatch
    a.add(s) // ERROR type mismatch
    a[0] = null // ERROR type mismatch
    a.contains(null) // ERROR must be mentioned in input types
    a.indexOf(null) // ERROR must be mentioned in input types
    val l1: MutableList<String?> = a // ERROR type mismatch
    val l2: List<String?> = a
    val l3: Collection<String?> = a
    val l4: java.util.AbstractList<String?> = a
    val l5: MutableList<String> = a
    a.own(null)
    val x: String = a[0]
    a.add("")
    a[0] = ""
    for (e in a) {
        val y: String = e
    }
}

fun jdk() {
    val r = ArrayList<String>()
    r.add(null) // ERROR type mismatch
    r.remove(null) // ERROR must be mentioned in input types
    r.addAll(listOf(null)) // ERROR addAll
    val m: MutableList<String?> = r // ERROR type mismatch
    val m2: List<String?> = r
    val h = HashMap<String, Int>()
    h.put(null, 1) // ERROR type mismatch
    h.get(null) // ERROR must be mentioned in input types
    h.containsKey(null) // ERROR must be mentioned in input types
    val hm: MutableMap<String?, Int> = HashMap<String, Int>() // ERROR type mismatch
    val hs: MutableSet<String?> = HashSet<String>() // ERROR type mismatch
    val v: Int? = h.get("")
}

// A subclass not annotated itself isn't pure, nor is a class whose
// annotation names a class that doesn't exist, or one with another
// number of type parameters.
fun notPure() {
    val b = B<String>()
    b.add(null)
    val bm: MutableList<String?> = b
    val c = C<String>()
    c.add(null)
    val cm: MutableList<String?> = c
    val i = I<String>()
    i.add(null)
    val im: MutableList<String?> = i
    val w = W<String, Int>()
    w.put(null, 1)
    val wm: MutableMap<String?, Int> = w
}

class K : A<String>() {
    override fun add(element: String): Boolean = true
}
