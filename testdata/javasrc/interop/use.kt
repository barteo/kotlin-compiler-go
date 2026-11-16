package interop

fun interopDemo() {
    println(FromJava().size2)
    val sized: Sized = FromJava()
    println(sized.size2)
    println(JavaDerived().label)
    println(JavaGetters().title)
    println(Tally().count)
    val counted: JavaCounted = Tally()
    println(counted.count)
    val d: Described = Description()
    println("${Description().text} ${d.text}")
    val outer = Outer("o")
    val inner: Outer<String>.Inner<Int> = outer.Inner<Int>(3)
    println("${inner.value} ${inner.extra}")
    println(Outer(5).Plain().describe())
    println(Outer<String>().Plain().describe())
    val o = Overloads()
    println(o.foo({ "got $it" }, ""))
    println(o.bar { "got $it" })
    println(o.test { })
    println(samOverload { "generic" })
    nothingFromJava(object : Nulls.Visitor<Nothing> {})
    println(StaticOuter().Inner().test())
    println(Sources().one("a").get() + Sources().current().get() + Sources().direct().get())
    println(Sources().longer("abc", 2).get() + " " + Sources().longer("a", 2).get() + " " + Sources().longer("", 2).get())
    println(Named().run())
    // A platform value isn't checked for null where a Java method takes
    // it, a parameter that an annotation makes non-null too.
    println(Names.show(Names.none()))
    val id = 5L
    val next: Long = Ids.next(id)
    val ids = Ids(3)
    ids.last = ids.get() + ids.last
    println("${Ids.show(id, 2L)} ${Ids.show(listOf(3L)[0], id)} ${next + Ids.next(next)} ${ids.last == 20L}")
    val job: Job<String> = FixedJob("a")
    println("${job.clone().run()} ${FixedJob("b").clone().run()} ${Leaf().step()} ${Both().step()}")
    for (c in listOf(FixedJob::class.java, Leaf::class.java, Both::class.java)) {
        for (m in c.declaredMethods.filter { !it.isBridge && (it.name == "clone" || it.name == "step") }) {
            println("${c.simpleName}.${m.name} ${java.lang.reflect.Modifier.toString(m.modifiers)}")
        }
    }
}

// An override without a visibility modifier is public where one of the
// functions it overrides is: the clone() of a Java interface that extends
// Cloneable, whose own clone() is protected, a function that a superclass
// makes public, and one that an interface declares too.
class FixedJob<T>(private val v: T) : Job<T> {
    override fun run(): T = v

    override fun clone(): Job<T> = FixedJob(v)
}

open class Step {
    protected open fun step(): String = "step"
}

open class Wide : Step() {
    public override fun step(): String = "wide"
}

class Leaf : Wide() {
    override fun step(): String = "leaf"
}

interface Stepper {
    fun step(): String
}

class Both : Step(), Stepper {
    override fun step(): String = "both"
}

// An annotated Java List or Map is a mutable collection and a read-only
// one: it takes a read-only list, an empty one of an inferred type too,
// and an override declares either.
class Named {
    fun run(): String {
        val names = Names(1, listOf("a", "b", "c"))
        var seen = 0
        names.notify(object : Names.Listener {
            override fun changed(names: List<String>) {
                seen += names.size
            }
        })
        names.notify(object : Names.Listener {
            override fun changed(names: MutableList<String>) {
                seen += names.size
            }
        })
        val all: MutableList<String> = names.all()
        all.add("d")
        val readOnly: List<String> = names.all()
        val provided = Names.provided(object : Names.Provider {
            override fun names(): List<String> = readOnly

            override fun more(): List<String>? = listOf("e")
        })
        val first: String = names.first()
        return "$first $seen ${Names.count(readOnly, mapOf("k" to "v"))} ${Names.count(emptyList(), null)} ${Names.count(all, emptyMap())} $provided"
    }
}

// T of a Java <T> Source<T> fromCallable(Callable<? extends T>), for a
// callable of a String? and an expected Source<String>, is a String!.
class Sources {
    private val items = hashMapOf("a" to "first")
    private var last: String? = " last"

    fun one(key: String): Source<String> {
        return Source.fromCallable(java.util.concurrent.Callable { items[key] })
    }

    fun current(): Source<String> = Source.fromCallable { last }

    fun direct(): Source<String> = Source.of(last)

    // The lambda for a Java Check<T>, whose test(@NonNull T) takes a
    // T & Any, takes a String for the String? that map's lambda returns.
    fun longer(s: String, n: Int): Source<String?> =
        Source.of(s)
            .map { v: String -> v.ifEmpty { null } }
            .filter { v: String -> v.length > n }
}

// foo() in an inner subclass of a Java class is its static foo(), which
// comes before the outer class's member foo().
class StaticOuter {
    fun foo() = "outer foo"
    inner class Inner : StaticBase() {
        fun test() = foo()
    }
}

// T of a Java <T> T set(String, T), for a null, is Nothing!, which takes
// null and returns: the code after the call runs.
fun nothingFromJava(v: Nulls.Visitor<Nothing>) {
    val n = Nulls()
    n.set("key", null)
    println("after set")
    val s: String? = n.set("key", null)
    println(s)
    println(n.accept(v, null) == null)
    println(n.remap<Int, String>(2) { k, v -> null })
}

fun <T> samOverload(action: () -> T): T = action()
fun samOverload(action: Runnable): String = "runnable"

