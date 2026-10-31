class Point(val x: Int, val y: Int) {
    fun manhattan(): Int = abs(x) + abs(y)
    fun plus(other: Point): Point = Point(x + other.x, y + other.y)
    fun describe(): String = "($x, $y)"
}

class Counter(start: Int, val name: String) {
    var count = start
    val label: String
        = "counter " + name
    var isActive: Boolean = true

    init {
        println("created $label at $start")
    }

    fun increment(by: Int) {
        count += by
    }

    fun incrementTwice() {
        increment(1)
        this.increment(1)
    }
}

class Account(val owner: String, var balance: Double) {
    val history: String = "opened by $owner"

    fun deposit(amount: Double): Boolean {
        if (amount <= 0.0) return false
        balance += amount
        return true
    }
}

fun abs(n: Int): Int = if (n < 0) -n else n

fun main() {
    val p = Point(3, -4)
    println(p.manhattan())
    println(p.plus(Point(1, 1)).describe())
    println(p.x + p.y)

    val c = Counter(10, "clicks")
    c.increment(5)
    c.incrementTwice()
    c.count++
    c.count *= 2
    println(c.count)
    println(c.isActive)
    c.isActive = false
    println(c.isActive)
    println(c.name)

    val a = Account("Ada", 100.0)
    println(a.deposit(50.5))
    println(a.deposit(-1.0))
    println(a.balance)
    a.balance = 0.0
    println(a.balance)
    println(a.history)
}
