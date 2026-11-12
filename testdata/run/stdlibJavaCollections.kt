// The JDK's collection classes seen with Kotlin's collection types'
// signatures: contains(E) for contains(Object), and the purely implemented
// classes' members without platform types. The calls and overrides still
// call and override the Java methods.
import java.util.*
import java.util.concurrent.ConcurrentHashMap

class Upper : ArrayList<String>() {
    override fun add(element: String): Boolean = super.add(element.uppercase())
    override fun contains(element: String): Boolean = super.contains(element.uppercase())
    override fun indexOf(element: String): Int = super.indexOf(element.uppercase())
    override fun remove(element: String): Boolean = super.remove(element.uppercase())
}

class Defaulting : HashMap<String, Int>() {
    override fun get(key: String): Int? = super.get(key) ?: -1
    override fun containsKey(key: String): Boolean = key == "any" || super.containsKey(key)
}

class Counted : LinkedHashSet<String>() {
    var adds = 0
    override fun add(element: String): Boolean {
        adds++
        return super.add(element)
    }
}

fun <E> firstOf(xs: LinkedList<E>): E = xs.removeFirst()

fun main() {
    val al = ArrayList<String>()
    al.add("a")
    al.add("b")
    val ml: MutableList<String> = al
    println(ml.contains("a"))
    println(al.indexOf("b"))
    println(al.lastIndexOf("z"))
    val s: String = al[0]
    println(s)
    println(al.subList(0, 1))
    println(al.removeAll(listOf("a")))
    println(al)

    val up = Upper()
    up.add("x")
    up.add("y")
    println(up)
    println(up.contains("x"))
    println(up.indexOf("y"))
    println(up.remove("x"))
    println(up)
    val asList: MutableList<String> = up
    println(asList.contains("y"))
    println((up as java.util.Collection<*>).contains("Y"))

    val hm = HashMap<String, Int>()
    hm["one"] = 1
    val old: Int? = hm.put("one", 11)
    println(old)
    println(hm.get("one"))
    println(hm["two"])
    println(hm.getOrDefault("two", 2))
    println(hm.containsKey("one"))
    println(hm.containsValue(11))
    println(hm.remove("one"))
    val keys: MutableSet<String> = hm.keys
    println(keys)

    val d = Defaulting()
    d["k"] = 3
    println(d["k"])
    println(d["missing"])
    println(d.containsKey("any"))
    val asMap: Map<String, Int> = d
    println(asMap["missing"])
    println(asMap.containsKey("any"))

    val c = Counted()
    c.add("p")
    c.add("p")
    println(c.adds)
    println(c.contains("p"))

    val ll = LinkedList(listOf(5, 6))
    println(firstOf(ll))
    ll.push(4)
    println(ll)

    val chm = ConcurrentHashMap<String, Int>()
    chm["a"] = 1
    println(chm.remove("a", 2))
    println(chm.remove("a", 1))

    val v = Vector<String?>()
    v.add(null)
    println(v.contains(null))
    val nums = ArrayList<Int>(listOf(10, 20, 30))
    println(nums.remove(20))
    println(nums)
}
