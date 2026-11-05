// a?.b = v, a?.b op= v and a?.b++ change b only if a isn't null.
class State(var value: String)

class Session(var lastFailed: String?, val state: State?)

class Manager(var active: Session?) {
    var last: String?
        get() = active?.lastFailed
        set(value) {
            active?.lastFailed = value
        }

    fun waiting() {
        active?.state?.value = "WAITING"
    }
}

class Version(val v: Int) {
    operator fun inc() = Version(v + 1)
    override fun toString() = "v$v"
}

class Counter(var n: Int, var ch: Char = 'a', var version: Version = Version(1))

class Holder(var counter: Counter?)

var reads = 0
val holder = Holder(null)

fun holder(): Holder {
    reads++
    return holder
}

fun main() {
    val m = Manager(Session(null, State("idle")))
    m.last = "x"
    m.waiting()
    println(m.last + m.active?.state?.value)
    val n = Manager(null)
    n.last = "y"
    n.waiting()
    println(n.last)
    var c: Counter? = Counter(1)
    c?.n += 2
    println(c?.n)
    c = null
    c?.n += 2
    println(c?.n)
    // a?.b++ and friends, as statements and as values, which are null
    // when a is.
    c?.n++
    --c?.n
    println("${c?.n++} ${++c?.n}")
    c = Counter(1)
    c?.n++
    ++c?.n
    c?.ch++
    c?.version++
    println("${c.n} ${c.ch} ${c.version}")
    val old: Int? = c?.n--
    val new = ++c?.n
    println("$old $new ${c.n}")
    // The receiver is evaluated once.
    holder()?.counter?.n++
    holder.counter = c
    holder().counter?.n++
    println("${holder().counter?.n--} ${c.n} $reads")
}
