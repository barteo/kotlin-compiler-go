// a?.b = v assigns b only if a isn't null.
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

class Counter(var n: Int)

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
}
