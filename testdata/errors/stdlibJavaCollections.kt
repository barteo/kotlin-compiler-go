// How Kotlin sees the JDK's collection classes: their members that take
// Object take the Kotlin collection type's type parameter instead, and
// the purely implemented ones (ArrayList, HashSet, HashMap, ...) have type
// arguments that aren't platform types.
import java.util.*
import java.util.concurrent.*

fun bar(): String? = null

fun pure() {
    val al = ArrayList<String>()
    al.add(null) // ERROR inferred type is Nothing? but String was expected
    al.add(bar()) // ERROR inferred type is String? but String was expected
    al.add("")
    al[0] = null // ERROR inferred type is Nothing? but String was expected
    al.subList(0, 1).add(null) // ERROR inferred type is Nothing? but String was expected
    val b1: MutableList<String?> = al // ERROR inferred type is ArrayList<String> but MutableList<String?> was expected
    val b2: MutableList<String> = al
    val b3: List<String?> = al
    val b4: MutableCollection<String?> = al // ERROR inferred type is ArrayList<String> but MutableCollection<String?> was expected

    val ll = LinkedList<String>()
    ll.addFirst(null) // ERROR inferred type is Nothing? but String was expected
    ll.push(null) // Deque's, which no Kotlin member declares: String!
    ll.offer(null)

    val lhs = LinkedHashSet<String>()
    lhs.add(null) // ERROR inferred type is Nothing? but String was expected
    val b5: MutableSet<String?> = lhs // ERROR inferred type is LinkedHashSet<String> but MutableSet<String?> was expected

    val hm = HashMap<String, Int>()
    hm.put(null, 1) // ERROR inferred type is Nothing? but String was expected
    hm.put("", null) // ERROR inferred type is Nothing? but Int was expected
    val old: Int = hm.put("", 1) // ERROR inferred type is Int? but Int was expected
    val keys: MutableSet<String?> = hm.keys // ERROR inferred type is MutableSet<String> but MutableSet<String?> was expected

    val tm = TreeMap<String, Int>()
    tm.put(null, 1) // ERROR inferred type is Nothing? but String was expected
    val first: String? = tm.firstKey() // SortedMap's: K!

    val chm = ConcurrentHashMap<String, Int>()
    chm.remove(null, 1) // ERROR type parameter 'K' must be mentioned in input types
    chm.putIfAbsent(null, 1) // ERROR inferred type is Nothing? but String was expected
}

fun erased(v: Vector<String>, w: WeakHashMap<String, Int>, cm: ConcurrentMap<String, Int>) {
    val al = ArrayList<String>()
    al.contains(1) // ERROR type parameter 'T' must be mentioned in input types
    al.indexOf(1) // ERROR type parameter 'T' must be mentioned in input types
    al.contains("")
    al.remove("")
    val hm = HashMap<String, Int>()
    hm.containsKey(1) // ERROR type parameter 'K' must be mentioned in input types
    hm.containsValue("") // ERROR type parameter 'V' must be mentioned in input types
    hm.getOrDefault(1, 2) // ERROR type parameter 'K' must be mentioned in input types
    hm.remove(1) // ERROR type parameter 'K' must be mentioned in input types
    hm[1] // ERROR type parameter 'K' must be mentioned in input types
    hm[""]

    // Classes that aren't purely implemented: platform type arguments.
    v.add(null)
    v.contains(null)
    v.contains(1) // ERROR type parameter 'T' must be mentioned in input types
    w.put(null, 1)
    w.containsKey(1) // ERROR type parameter 'K' must be mentioned in input types
    cm.remove("", 1)
    cm.remove(null, null)
    cm.remove("", "") // ERROR type parameter 'V' must be mentioned in input types
}

class L1 : ArrayList<String>() {
    override fun add(element: String?): Boolean = true // ERROR 'add' overrides nothing
    override fun contains(element: String?): Boolean = true // ERROR 'contains' overrides nothing
    override fun get(index: Int): String? = null // ERROR return type of 'get' is not a subtype
}

class L2 : ArrayList<String>() {
    override fun add(element: String): Boolean = true
    override fun contains(element: String): Boolean = true
    override fun remove(element: String): Boolean = true
}

class M1 : HashMap<String, Int>() {
    override fun put(key: String, value: Int): Int? = null
    override fun get(key: String): Int? = null
    override fun containsKey(key: String?): Boolean = true // ERROR 'containsKey' overrides nothing
}

class V1 : Vector<String>() {
    override fun add(element: String?): Boolean = true
    override fun contains(element: String?): Boolean = true
}
