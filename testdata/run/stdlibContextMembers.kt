// Context parameters beyond top-level functions: members, which take them
// after the object, in classes, interfaces and objects, overridden, with a
// generic override's bridge, delegated, and called on this, safely and with
// default values; member extensions; local functions, which take them
// before the receiver and the captures; properties, top-level, extension
// and member, abstract, generic, read, assigned, compound-assigned and
// safely assigned, whose accessors take them; anonymous functions with
// context parameters; fun interfaces whose method has them; tailrec
// functions; contextOf with an explicit type argument; and a Unit context.

class Logger(val name: String) {
    val lines = mutableListOf<String>()
    fun log(s: String) { lines += "$name: $s" }
}

class Counter {
    var n = 0
}

interface Greeter {
    context(logger: Logger)
    fun greet(who: String): String

    context(logger: Logger)
    fun hello(): String = "hello from ${logger.name}"
}

open class Base : Greeter {
    context(logger: Logger)
    override fun greet(who: String): String {
        logger.log("greet $who")
        return "hi $who"
    }

    context(logger: Logger)
    open fun twice(s: String, times: Int = 2): String = s.repeat(times) + " " + logger.name

    context(logger: Logger)
    fun String.tagged(): String = "<$this ${logger.name}>"

    fun useTagged(): String = with(Logger("member")) { "x".tagged() }
}

class Derived : Base() {
    context(logger: Logger)
    override fun twice(s: String, times: Int): String = "derived " + super.twice(s, times)
}

interface Source<T> {
    context(logger: Logger)
    fun get(): T
}

object Strings : Source<String> {
    context(logger: Logger)
    override fun get(): String = "from ${logger.name}"
}

class Delegating(g: Greeter) : Greeter by g

fun interface Action {
    context(logger: Logger)
    fun run(n: Int): String
}

context(logger: Logger)
val loggerName: String
    get() = logger.name

context(counter: Counter)
var count: Int
    get() = counter.n
    set(value) {
        counter.n = value
    }

context(logger: Logger)
val String.shouted: String
    get() = uppercase() + "!" + logger.name

class Box<T>(val value: T)

context(box: Box<T>)
val <T> boxed: T
    get() = box.value

interface Named {
    context(logger: Logger)
    val label: String
}

class Item(val id: Int) : Named {
    context(logger: Logger)
    override val label: String
        get() = "item $id of ${logger.name}"

    context(counter: Counter)
    var total: Int
        get() = counter.n * 10
        set(value) {
            counter.n = value / 10
        }
}

context(logger: Logger)
tailrec fun countdown(n: Int, acc: String = ""): String = if (n == 0) acc + logger.name else countdown(n - 1, "$acc$n ")

context(u: Unit, s: String)
fun Int.withUnit(): String = "$this $u $s"

fun locals(): String {
    var result = ""
    val captured = "captured"

    fun foo(a: Logger) = "value"

    fun Logger.foo() = "extension"

    context(logger: Logger)
    fun foo() = "context"

    context(logger: Logger)
    fun String.local(times: Int = 1): String = repeat(times) + " " + logger.name + " " + captured

    with(Logger("local")) {
        result += foo() + " "
        result += "s".local() + " " + "t".local(2)
    }
    return result + " " + Logger("x").foo() + " " + foo(Logger("y"))
}

fun main() {
    val logger = Logger("main")
    with(logger) {
        val b: Base = Derived()
        println(b.greet("you"))
        println(b.hello())
        println(b.twice("ab"))
        println(b.twice("ab", 3))
        println(b.useTagged())
        val g: Greeter? = b
        println(g?.greet("safe"))
        val s: Source<String> = Strings
        println(s.get())
        println(Delegating(Derived()).greet("delegated"))
        println(countdown(3))
        println(loggerName + " " + "abc".shouted)
        val item: Named = Item(7)
        println(item.label)
    }
    println(logger.lines)

    val counter = Counter()
    with(counter) {
        count = 2
        count += 3
        count++
        println(count)
        val item = Item(1)
        item.total = 70
        item.total += 10
        val maybe: Item? = item
        maybe?.total = 120
        println("${item.total} ${counter.n}")
    }

    with(Box("boxed")) { println(boxed) }

    val anon = context(l: Logger) fun(x: Int): String = "anon $x ${l.name}"
    println(anon(Logger("explicit"), 1))
    with(Logger("implicit")) { println(anon(2)) }

    val action = Action { n -> "action $n ${contextOf<Logger>().name}" }
    with(Logger("sam")) { println(action.run(3)) }

    println(locals())
    with(Unit) { with("s") { println(4.withUnit()) } }
}
