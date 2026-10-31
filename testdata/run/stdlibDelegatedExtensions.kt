// Delegated extension properties, whose delegate's thisRef is the
// receiver, top level and in classes, and properties delegated to
// property references, val y by ::x, whose getValue and setValue are the
// standard library's. In a class, ::x is this::x, whose field a member
// extension property reads from this, not its receiver.
import kotlin.reflect.KProperty

class Tagged(private val tag: String) {
    var log = ""

    operator fun getValue(thisRef: Any?, p: KProperty<*>): String {
        log += "get ${p.name} of $thisRef;"
        return "$tag:$thisRef"
    }

    operator fun setValue(thisRef: Any?, p: KProperty<*>, v: String) {
        log += "set ${p.name} of $thisRef to $v;"
    }
}

class Counter {
    private val counts = HashMap<Any, Int>()

    operator fun getValue(thisRef: Any, p: KProperty<*>): Int = counts[thisRef] ?: 0

    operator fun setValue(thisRef: Any, p: KProperty<*>, v: Int) {
        counts[thisRef] = v
    }
}

class A(val name: String) {
    override fun toString() = name
}

class B

val topTag = Tagged("top")
var String.shout: String by topTag
var A.count: Int by Counter()

// Two fields, size$delegate and size$delegate$1.
val A.size: Int by lazy { 1 }
val B.size: Int by lazy { 2 }

class Host(private val prefix: String) {
    val tag = Tagged(prefix)
    var Int.label: String by tag

    operator fun String.provideDelegate(thisRef: Host, p: KProperty<*>): Tagged {
        println("provide ${p.name} for ${thisRef.prefix}")
        return Tagged(this)
    }

    val A.greeting: String by "hello"

    fun run(a: A): String {
        5.label = "five"
        return 7.label + " " + a.greeting
    }
}

class Point(var x: Int, val y: Int)

val origin = Point(1, 2)
var px by origin::x
val py: Int by origin::y
var Point.left: Int by Point::x

class Holder(val p: Point) {
    val doubled by p::y
}

open class Named(val name: String) {
    var count = 0
}

class Self(name: String) : Named(name) {
    val Self.alias: String by ::name
    var tally by ::count

    fun aliasOf(other: Self) = other.alias
}

fun main() {
    println("abc".shout)
    "abc".shout = "ABC"
    println(topTag.log)

    val a = A("a")
    val b = A("b")
    a.count += 3
    b.count++
    a.count++
    println("${a.count} ${b.count}")

    println("${a.size} ${B().size}")

    val h = Host("h")
    println(h.run(a))
    println(h.tag.log)

    px = 10
    println("$px $py ${origin.x}")
    val q = Point(3, 4)
    q.left += 5
    println("${q.x} ${q.left} ${Holder(q).doubled}")

    val local by origin::y
    var localX by q::x
    localX = 42
    println("$local ${q.x}")

    val s = Self("s")
    s.tally += 2
    println("${s.aliasOf(Self("other"))} ${s.tally} ${s.count}")
}
