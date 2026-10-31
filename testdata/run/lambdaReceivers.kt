// Function types with receivers and lambdas with receivers, which are
// objects here: their receiver is the first parameter.

class Tag(val name: String) {
    val children = java.util.ArrayList<Tag>()
    var text = ""

    fun tag(name: String, build: Tag.() -> Unit): Tag {
        val t = Tag(name)
        t.build()
        children.add(t)
        return t
    }

    fun render(): String {
        val sb = StringBuilder()
        sb.append("<").append(name).append(">").append(text)
        for (c in children) {
            sb.append(c.render())
        }
        return sb.append("</").append(name).append(">").toString()
    }
}

fun html(build: Tag.() -> Unit): Tag {
    val t = Tag("html")
    build(t)
    return t
}

fun repeatOn(sb: StringBuilder, n: Int, action: StringBuilder.(Int) -> Unit) {
    var i = 0
    while (i < n) {
        sb.action(i)
        i++
    }
}

class Counter(var n: Int) {
    // A lambda with a receiver in a method: this is the receiver,
    // this@Counter the counter.
    fun addTo(sb: StringBuilder): StringBuilder {
        repeatOn(sb, 2) { i -> append(i).append(this@Counter.n).append(length) }
        return sb
    }
}

fun main() {
    val page = html {
        text = "t"
        tag("body") {
            tag("p") { text = "hi" }
            text = name.uppercase()
        }
    }
    println(page.render())

    val sb = StringBuilder()
    repeatOn(sb, 3) { i -> append(i).append(';') }
    println(sb)

    val f: StringBuilder.(String) -> StringBuilder = { s -> append(s).append(this.length) }
    println(f(StringBuilder("x"), "y"))
    println(StringBuilder("a").f("b"))
    // A function type with a receiver is assignable to one without.
    val g: (StringBuilder, String) -> StringBuilder = f
    println(g(StringBuilder(), "z"))
    println(Counter(7).addTo(StringBuilder()))

    // Nested receivers: the innermost one comes first.
    val outer = html {
        tag("a") {
            text = this@html.name + "/" + name
        }
    }
    println(outer.render())
}
