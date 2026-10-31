package app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.ComposeNode
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.NonRestartableComposable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.State
import androidx.compose.runtime.currentComposer
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import lib.Box
import lib.ForEach
import lib.Measure
import lib.Node
import lib.NodeApplier
import lib.Row
import lib.Divider
import lib.Insets
import lib.Label
import lib.Scope
import lib.Text
import lib.Twice
import lib.Widget
import lib.label
import lib.visible

// Composable functions of every shape, which use/ calls.

@Composable
fun Greeting(name: String) {
    Text("Hello, $name")
}

// Reads state of its own, so it recomposes alone when the state changes.
@Composable
fun Counter(count: State<Int>) {
    Text("count ${count.value}")
}

@Composable
fun Defaults(a: String, b: String = "b", c: Int = a.length) {
    Text("$a $b $c")
    Label(a)
    Label(a, suffix = b)
}

@Composable
fun accent(): String = "accent"

// Default values that call composables, which the body computes.
@Composable
fun Themed(text: String, color: String = accent(), id: Int = remember { nextId() }) {
    Text("$text $color ${id > 0}")
}

// More parameters than one $changed int has slots for.
@Composable
fun Many(p0: Int, p1: Int, p2: Int, p3: Int, p4: Int, p5: Int, p6: Int, p7: Int, p8: Int, p9: Int, p10: Int, p11: String = "end") {
    Text("${p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10} $p11")
}

@Composable
fun String.Shout() {
    Text(uppercase() + "!")
}

@Composable
fun describe(n: Int): String = "n=$n"

@Composable
@ReadOnlyComposable
fun twice(s: String): String = s + s

@Composable
@NonRestartableComposable
fun Plain(s: String) {
    Text("plain $s")
}

val title: String
    @Composable get() = "title"

// Prints when it enters the composition, and when it leaves it or its
// name changes.
@Composable
fun Tracked(name: String) {
    DisposableEffect(name) {
        println("enter $name")
        onDispose { println("leave $name") }
    }
    Text(name)
}

class Panel(val heading: String) {
    @Composable
    fun Show(suffix: String = "!") {
        Text(heading + suffix)
        "in".In()
    }

    @Composable
    fun String.In() {
        Text("$heading/$this")
    }
}

class Fancy : Widget {
    @Composable
    override fun Render(label: String) {
        Text("fancy $label")
    }
}

// An interface whose composable use/ implements.
interface Item {
    @Composable
    fun Draw(n: Int)
}

// Returns early: the restart group still ends.
@Composable
fun Early(skip: Boolean) {
    if (skip) {
        return
    }
    Text("not skipped")
}

object Widgets {
    @Composable
    fun Wide(l: Long, d: Double, vararg names: String) {
        Text("$l $d ${names.joinToString()}")
    }
}

@Composable
fun Int.Times(label: String = "x", sep: String = "*") {
    Text("$this$sep$label")
}

// A generic interface, whose override in IntSlot needs a bridge.
interface Slot<T> {
    @Composable
    fun Put(value: T)
}

class IntSlot : Slot<Int> {
    @Composable
    override fun Put(value: Int) {
        Text("slot $value")
    }
}

@Composable
fun DrawAll(a: Item, b: Item) {
    a.Draw(1)
    b.Draw(2)
}

// Composable function types and lambdas, which use/ passes its lambdas
// to and gets the program's from.

@Composable
fun Frame(title: String, content: @Composable () -> Unit) {
    Text("[$title]")
    content()
}

@Composable
fun Both(first: String, second: String, item: @Composable (String) -> Unit) {
    item(first)
    item(second)
}

@Composable
fun Scoped(block: @Composable String.(Int) -> Unit) {
    "scope".block(1)
}

// A composable lambda that returns a value runs in a group of its own.
@Composable
fun Compute(f: @Composable (Int) -> String): String = f(2)

@Composable
fun Maybe(content: (@Composable () -> Unit)?) {
    Text("maybe")
    content!!()
}

@Composable
fun Framed(content: @Composable () -> Unit = { Text("default content") }) {
    Frame("framed", content)
}

// Composable lambdas outside composables.
val Header: @Composable () -> Unit = { Text("header") }

fun footer(n: Int): @Composable () -> Unit = { Text("footer $n") }

class Holder(val content: @Composable () -> Unit) {
    @Composable
    fun Show() {
        Frame("holder", content)
    }
}

@Composable
fun Lambdas(name: String, n: Int) {
    Frame("frame") { Text("in frame $name") }
    Box("box") {
        Text("in lib's box")
        Greeting(name)
    }
    Both("a", "b") { s -> Text("item $s$n") }
    Scoped { i -> Text("$this $i $name") }
    Scope { i -> Text("$this $i") }
    Twice { Text("twice $it") }
    Text(Compute { "computed ${it + n}" })
    Text(Compute { x -> if (x > n) return@Compute "early $x"; "late $x" })
    Header()
    footer(n)()
    label(name)()
    Divider()
    Text("insets ${Insets.visible}")
    Holder { Text("held $n") }.Show()
    val local: @Composable () -> Unit = { Text("local $n") }
    local()
    local.invoke()
    Frame("nested") { Frame("inner") { Text("deep $name") } }
    val annotated = @Composable { Text("annotated") }
    annotated()
    Maybe { Text("not null") }
    Framed()
    Framed { Text("given content") }
    // Remembered: it enters once, and leaves when n changes.
    Frame("tracked") { Tracked("lambda $n") }
}

// Inline composables, remember and key, which use/ calls too.

var ids = 0

fun nextId() = ++ids

@Composable
inline fun Wrap(label: String, content: @Composable () -> Unit) {
    Text("<$label>")
    content()
}

// The inlined lambda returns a value: it runs in a group of its own.
@Composable
inline fun Pick(n: Int, sep: String = ":", f: @Composable (Int) -> String): String = "pick" + sep + f(n)

@Composable
inline fun String.Each2(item: @Composable (String) -> Unit) {
    item(this)
    item("$this!")
}

@Composable
inline fun <reified T> TypeName() {
    Text("type ${T::class.java.simpleName}")
}

object Layout {
    @Composable
    inline fun Column(label: String, content: @Composable () -> Unit) {
        Text("column $label")
        Wrap(label, content)
    }
}

// Lib's inline ComposeNode, with a value class receiver and a reified
// applier.
@Composable
fun Leaf(text: String) {
    ComposeNode<Node, NodeApplier>(factory = { Node("leaf") }, update = { set(text) { this.text = it } })
}

// Remembers an id, which stays while its group does.
@Composable
fun Keeps(label: String) {
    val id = remember { nextId() }
    Text("$label #$id")
}

@Composable
fun Inlines(name: String, n: Int) {
    Wrap("wrap") { Text("wrapped $name") }
    Text(Pick(n) { "picked $it" })
    Text(Pick(n, sep = "=") { x -> if (x > 1) return@Pick "big $x"; "small $x" })
    "each".Each2 { Text("each $it") }
    TypeName<Panel>()
    Layout.Column("col") { Keeps("in column") }
    Row {
        Text("in row $n")
        Keeps("in row")
    }
    Leaf("leaf $name")
    Text(Measure { "measured ${remember { nextId() }}" })
    Text(Measure { if (n > 1) return@Measure "early $n"; "late $n" })
    Frame("inline in lambda") { Wrap("inner") { Keeps("in lambda") } }
    val kept = remember { nextId() }
    val byN = remember(n) { nextId() }
    val both = remember(name, n) { "$name/$n#${nextId()}" }
    val first by remember { mutableStateOf(n) }
    Text("kept $kept, by n $byN, both $both, first $first")
    Text("composer ${currentComposer === currentComposer}")
    Keys(n > 1)
}

// Keyed groups move, with what they remember, when their order changes:
// none leaves, and none enters again.
@Composable
fun Keys(flip: Boolean) {
    val order = if (flip) listOf("y", "x") else listOf("x", "y")
    ForEach(order) { s ->
        key(s) {
            Keeps("key $s")
            Tracked("key $s")
        }
    }
    val joined = key(order[0], order[1], 3) { "joined #${remember { nextId() }}" }
    Text(joined)
}

// Control flow: the branches that call composables run in groups of their
// own, so what one remembers, or adds to the composition, isn't another's.

@Composable
fun flag(label: String): Boolean {
    Keeps(label)
    return true
}

// Returns a value, and early: its body runs in a group of its own.
@Composable
fun Returns(n: Int): Int {
    Keeps("before return")
    if (n > 1) {
        return n
    }
    Keeps("after return")
    return 0
}

@Composable
fun ReturnsUnit(n: Int) {
    Keeps("r first")
    if (n > 1) {
        Keeps("r in")
        return
    }
    Tracked("r after")
}

@Composable
@NonRestartableComposable
fun Plainly(on: Boolean) {
    if (on) Keeps("plainly on")
    if (!on) return
    Keeps("plainly after")
}

// Calls composables in a loop, and returns from it.
@Composable
inline fun Until(n: Int, item: @Composable (Int) -> Unit) {
    for (i in 0 until 4) {
        if (i == n) return
        item(i)
    }
}

@Composable
fun Total(xs: List<Int>): Int {
    var t = 0
    for (x in xs) t += remember(x) { x * 10 }
    return t
}

// Leaves a lambda of forEach for the function.
@Composable
fun EachReturn(n: Int) {
    Text("each start")
    listOf(1, 2, 3).forEach {
        if (it == 2 && n > 1) return
        Keeps("each ret $it")
    }
    Text("each end")
}

val flowTitle: String
    @Composable get() = if (ids > 0) describe(ids % 1) else "none"

@Composable
fun Flow(name: String, n: Int) {
    val on = n > 1
    if (on) Keeps("then") else Keeps("else")
    if (on) {
        Tracked("if on")
    }
    val v = if (on) remember { nextId() } else -1
    Text("value $v")
    when {
        on -> Keeps("when on")
        n == 0 -> Text("never")
        else -> Keeps("when off")
    }
    when (n) {
        1 -> Keeps("one")
        5 -> Tracked("five")
    }
    Text(if (on) describe(1) else "plain")
    // A shrinking loop: its iterations' groups go, and the remember
    // after it keeps its value; the remembers in its body don't have
    // groups of their own, as with kotlinc.
    for (i in 0 until 6 - n) Keeps("loop $i")
    val after = remember { nextId() }
    Text("after loop #$after")
    for (i in 0 until 3 - n.coerceAtMost(2)) {
        val r = remember { nextId() }
        Text("in loop #$r")
    }
    val after2 = remember { nextId() }
    Text("after remembers #$after2")
    listOf("a", "b").forEach { Text("each $it") }
    listOf("c", "d").forEach { Keeps("each $it") }
    var i = 0
    while (i < n) {
        if (i == 3) break
        Keeps("while $i")
        i++
    }
    for (j in 0..5) {
        if (j == n) {
            Keeps("break $j")
            break
        }
        Text("j $j")
    }
    for (j in 0..3) {
        if (j % 2 == 0) {
            Keeps("continue $j")
            continue
        }
        Text("odd $j")
    }
    val s: String? = if (on) null else name
    Text(s ?: describe(n))
    s?.Shout()
    s?.let { Keeps("let $it") }
    if (on && flag("and")) Text("and")
    if (on || flag("or")) Text("or")
    Frame("branchy") { if (on) Keeps("lambda on") else Text("lambda off") }
    Text(Measure { if (on) "measure ${remember { nextId() }}" else "measure off" })
    key(on) {
        if (on) Keeps("key on")
    }
    Text("returns ${Returns(n)}")
    ReturnsUnit(n)
    Plainly(on)
    Until(n) { Keeps("until $it") }
    Text("total ${Total(listOf(1, n))}")
    EachReturn(n)
    Text(flowTitle)
}

@Composable
fun Screen(name: String, n: Int) {
    Greeting(name)
    Defaults("x")
    Defaults("yy", "z")
    Defaults("w", c = n)
    Many(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, n)
    Many(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, "last")
    name.Shout()
    Text(describe(n))
    Text(twice(name))
    Plain(name)
    Text(title)
    Tracked("t$n")
    Panel("panel").Show()
    Fancy().Render(name)
    val slot: Slot<Int> = IntSlot()
    slot.Put(n)
    Early(n > 1)
    Widgets.Wide(1L shl 40, 0.5, "a", "b")
    Widgets.Wide(d = 1.5, l = n.toLong())
    n.Times()
    n.Times(sep = "x", label = "times")
    Lambdas(name, n)
    Inlines(name, n)
    Flow(name, n)
    Themed("themed")
    Themed("plain", "red", n)
}
