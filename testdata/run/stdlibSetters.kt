// Property setters and backing fields: field in accessors, setters with
// bodies, default setters with other visibilities, and properties whose
// accessors are custom with a field.

class Temperature {
    var celsius = 0.0
        set(value) {
            require(value >= -273.15) { "below absolute zero" }
            field = value
        }
    var fahrenheit: Double
        get() = celsius * 9 / 5 + 32
        set(value) {
            celsius = (value - 32) * 5 / 9
        }
}

class Counter {
    var count = 0
        private set
    var hits = 0
        internal set
    var label = "c"
        protected set
    fun inc() {
        count++
        hits += 2
        label += "+"
    }
    fun later(): Runnable = Runnable { count += 10 }
}

open class Base {
    open var size = 1
        set(value) {
            println("base set " + value)
            field = value
        }
}

class Derived : Base() {
    override var size = 2
        set(value) {
            println("derived set " + value)
            field = value * 10
        }
}

class Lazy {
    private var computed = 0
    var value: String = ""
        get() {
            if (field.isEmpty()) {
                computed++
                field = "v" + computed
            }
            return field
        }
        private set
    val fixed: Int
        get() = field + 1
    init {
        fixed = 5
    }
    val twice: Int = 21
        get() = field * 2
    var trimmed: String = ""
        get() = field.trim()
        set(v) {
            field = v + " "
        }
    fun count() = computed
}

var topSet = 0
    set(value) {
        field = value + 1
    }
var topHidden = 5
    private set
val topGetter: Int = 3
    get() = field + 100

var StringBuilder.lastChar: Char
    get() = toString().last()
    set(value) {
        setCharAt(length - 1, value)
    }

var <T> MutableList<T>.second: T
    get() = this[1]
    set(v) {
        this[1] = v
    }

var IntArray.total: Int
    get() = sum()
    set(value) {
        this[0] += value - sum()
    }

class Scaled(val factor: Int) {
    var IntArray.scaled: Int
        get() = this[0] * factor
        set(v) {
            this[0] = v / factor
        }
    fun run(a: IntArray): Int {
        a.scaled = 30
        a.scaled += 6
        return a[0]
    }
}

fun bumpHidden() {
    topHidden++
}

object Settings {
    var mode = "auto"
        set(value) {
            field = value.uppercase()
        }
}

class Names {
    private val items = ArrayList<String>()
    var last: String? = null
        private set
    fun add(s: String) {
        items.add(s)
        last = s
    }
}

fun main() {
    val t = Temperature()
    t.celsius = 100.0
    println(t.fahrenheit)
    t.fahrenheit = 32.0
    println(t.celsius)
    try {
        t.celsius = -300.0
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    val c = Counter()
    c.inc()
    c.inc()
    c.later().run()
    println(c.count.toString() + " " + c.hits + " " + c.label)
    c.hits = 7
    println(c.hits)
    val b: Base = Derived()
    b.size = 3
    println(b.size)
    b.size++
    println(b.size)
    val l = Lazy()
    println(l.value + l.value + l.count())
    println(l.twice + l.fixed)
    l.trimmed = "  x"
    println("[" + l.trimmed + "]")
    topSet = 1
    topSet += 1
    println(topSet)
    bumpHidden()
    println(topHidden)
    println(topGetter)
    Settings.mode = "manual"
    println(Settings.mode)
    val n = Names()
    n.add("a")
    n.add("b")
    println(n.last)
    val sb = StringBuilder("abc")
    sb.lastChar = 'z'
    println(sb)
    val xs = mutableListOf(1, 2, 3)
    xs.second = 20
    xs.second += 1
    println(xs)
    val a = intArrayOf(1, 2)
    a.total = 10
    a.total++
    println(a.toList())
    println(Scaled(3).run(intArrayOf(0)))
    val m = Counter::class.java.methods.map { it.name }.filter { it.startsWith("set") }.sorted()
    println(m)
}
