interface Named {
    val name: String
    fun greet(): String
    fun hello(): String = "hello " + name
}

@JvmInline
value class Code(val code: Int) {
    init {
        if (code < 0) throw IllegalArgumentException("negative: " + code)
    }

    fun twice(): Code = Code(code * 2)

    val isZero: Boolean
        get() = code == 0

    fun plus(other: Code): Code = Code(code + other.code)

    fun describe(): String = "code " + code + (if (isZero) " (zero)" else "")
}

@JvmInline
value class Name(override val name: String) : Named {
    override fun greet(): String = "hi " + name

    companion object {
        fun of(s: String): Name = Name(s.uppercase())
    }
}

@JvmInline
value class Items private constructor(val items: java.util.ArrayList<String>) {
    fun size(): Int = items.size

    companion object {
        fun of(s: String): Items {
            val l = java.util.ArrayList<String>()
            l.add(s)
            return Items(l)
        }
    }
}

fun take(c: Code): Int = c.code

fun takeNullable(c: Code?): Int = if (c == null) -1 else c.code

fun takeName(n: Name?): String = if (n == null) "-" else n.name

fun make(i: Int): Code = Code(i)

fun both(a: Code, b: Name, x: Int): String = "" + a.code + b.name + x

fun anyOf(a: Any): String = a.toString()

fun <T> id(t: T): T = t

fun String.toCode(): Code = Code(length)

fun Code.next(): Code = Code(code + 1)

class Holder(val c: Code, var n: Name?) {
    fun get(): Code = c
}

data class Both(val a: Code, val b: Name)

interface Labeled {
    fun label(): String
}

@JvmInline
value class Label(val text: String) : Labeled {
    override fun label(): String = "<" + text + ">"
}

// Delegation to a value class: the delegate is boxed.
class Widget(val l: Label) : Labeled by l

class Gadget(l: Label) : Labeled by l

// The public methods of a class, as kotlinc names them.
fun methods(c: Class<*>) {
    val ms = c.declaredMethods
    val names = arrayOfNulls<String>(ms.size)
    var n = 0
    for (m in ms) {
        if (!m.isSynthetic() && java.lang.reflect.Modifier.isPublic(m.modifiers)) {
            names[n] = m.name + java.util.Arrays.toString(m.parameterTypes) + ": " + m.returnType.name
            n++
        }
    }
    java.util.Arrays.sort(names, 0, n)
    println(c.name + ":")
    var i = 0
    while (i < n) {
        println("  " + names[i])
        i++
    }
}

fun main() {
    val c = Code(3)
    println(take(c))
    println(takeNullable(null))
    println(takeNullable(c))
    println(make(4).twice().code)
    println(c.plus(Code(10)).code)
    println(both(c, Name("n"), 1))
    println(anyOf(c))
    println(c)
    println("in a template: $c")
    println(c == Code(3))
    println(c == Code(4))
    println(c.describe())
    println(Code(0).describe())
    val h = Holder(c, Name("x"))
    println(h.get().code)
    println(h.c)
    println(takeName(h.n))
    h.n = null
    println(takeName(h.n))
    val n: Named = Name("q")
    println(n.greet())
    println(n.hello())
    println(n.name)
    println(Name.of("w").greet())
    println(Name("v").hello())
    println(Items.of("s").size())
    println(id(c).code)
    val a: Any = Code(7)
    if (a is Code) {
        println(a.code + 1)
    }
    println((a as Code).twice())
    println("abc".toCode().next())
    val b = Both(Code(1), Name("p"))
    println(b)
    println(b == Both(Code(1), Name("p")))
    println(b.hashCode() == Both(Code(1), Name("p")).hashCode())
    val list = java.util.ArrayList<Code>()
    list.add(c)
    list.add(Code(5))
    println(list.get(1).code)
    println(list)
    try {
        Code(-1)
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(c.hashCode())
    val maybe: Name? = Name("maybe")
    println(maybe?.greet())
    val cc: Code? = Code(9)
    println(cc?.twice())
    println(cc!!.code)
    println(Widget(Label("w")).label())
    println(Gadget(Label("g")).label())
    methods(Class.forName("ValueClassesKt"))
    methods(Code::class.java)
    methods(Name::class.java)
    methods(Holder::class.java)
    methods(Both::class.java)
    methods(Widget::class.java)
}
