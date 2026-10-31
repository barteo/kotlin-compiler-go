package app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DontMemoize
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.NonSkippableComposable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.State
import androidx.compose.runtime.remember
import lib.Box
import lib.Counted
import lib.Node
import lib.Runs
import lib.Text

// Skipping: the composables count the runs of their bodies (lib.Runs),
// which show what recomposes, and what skips, when what Skips reads
// changes.

// Classes of every stability.
class Mutable(var v: Int)

data class Point(val x: Int, val y: Int)

data class Bag(val items: List<String>)

open class Base(val a: Int)

class Derived(a: Int, val b: Int) : Base(a)

@Stable
class Marked(var v: Int)

@Immutable
class Frozen(val v: Int)

enum class Mode { On, Off }

@JvmInline
value class Meters(val v: Int)

data class Holds<T>(val t: T)

class Pair2<A, B>(val a: A, val b: B)

interface Shape {
    val name: String
}

class Circle(override val name: String) : Shape

data class Square(override val name: String) : Shape

object Origin

class Lazy2 {
    val x by lazy { 1 }
}

data class MutData(var v: Int)

@Composable
fun Strings(s: String, n: Int) {
    Runs.hit("Strings")
    Text("strings $s $n")
}

@Composable
fun Unused(s: String, unused: Int) {
    Runs.hit("Unused")
    Text("unused $s")
}

@Composable
fun Mutables(m: Mutable) {
    Runs.hit("Mutables")
    Text("mutable ${m.v}")
}

@Composable
fun Points(p: Point) {
    Runs.hit("Points")
    Text("point $p")
}

@Composable
fun Bags(b: Bag, l: List<String>) {
    Runs.hit("Bags")
    Text("bag ${b.items} $l")
}

@Composable
fun Bases(b: Base, d: Derived) {
    Runs.hit("Bases")
    Text("base ${b.a} ${d.b}")
}

@Composable
fun Marks(m: Marked, f: Frozen) {
    Runs.hit("Marks")
    Text("marks ${m.v} ${f.v}")
}

@Composable
fun Modes(m: Mode, d: Meters, o: Origin) {
    Runs.hit("Modes")
    Text("modes $m ${d.v} ${o === Origin}")
}

@Composable
fun Holders(a: Holds<String>, b: Holds<Mutable>, c: Pair2<String, Int>) {
    Runs.hit("Holders")
    Text("holders ${a.t} ${b.t.v} ${c.a}")
}

@Composable
fun Shapes(s: Shape) {
    Runs.hit("Shapes " + s.name)
    Text("shape ${s.name}")
}

@Composable
fun <T> Anything(t: T, tag: String) {
    Runs.hit("Anything $tag")
    Text("anything $tag")
}

@Composable
fun Nodes(n: Node, l: Lazy2) {
    Runs.hit("Nodes")
    Text("node ${n.name} ${l.x}")
}

@Composable
fun Panels(p: Panel, tag: String) {
    Runs.hit("Panels $tag")
    Text("panel ${p.heading}")
}

fun prefix() = "p"

@Composable
fun Defaulted(s: String = "d", t: String = prefix() + "!") {
    Runs.hit("Defaulted")
    Text("defaulted $s $t")
}

@Composable
fun Varargs(vararg xs: String) {
    Runs.hit("Varargs")
    Text("varargs ${xs.joinToString()}")
}

@Composable
fun String.Suffixed(n: Int) {
    Runs.hit("Suffixed")
    Text("$this $n")
}

class Card(val title: String) {
    @Composable
    fun Show(n: Int) {
        Runs.hit("Card")
        Text("card $title $n")
    }

    @Composable
    fun Plain(n: Int) {
        Runs.hit("Card plain")
        Text("card plain $n")
    }
}

class Board(var title: String) {
    @Composable
    fun Show(n: Int) {
        Runs.hit("Board")
        Text("board $title $n")
    }
}

@Composable
fun Clicks(tag: String, onClick: () -> Unit) {
    Runs.hit("Clicks $tag")
    Text("clicks $tag")
}

@Composable
fun Content(tag: String, content: @Composable () -> Unit) {
    Runs.hit("Content $tag")
    content()
}

@Composable
fun Items(tag: String, item: @Composable (String) -> Unit) {
    Runs.hit("Items $tag")
    item("one")
    item(tag)
}

@Composable
@NonSkippableComposable
fun Always(s: String) {
    Runs.hit("Always")
    Text("always $s")
}

// Use their parameters: they recompose when an argument changed.
@Composable
fun Calls(tag: String, onClick: () -> Unit) {
    Runs.hit("Calls $tag")
    if (tag.isEmpty()) onClick()
    Text("calls $tag")
}

@Composable
fun <T> Shows(t: T, tag: String) {
    Runs.hit("Shows $tag")
    Text("shows $tag $t")
}

// Inline composables: their copies pass on what the caller said of their
// parameters, and the lambdas they invoke of theirs.
@Composable
inline fun <T> InlineShows(t: T, tag: String) {
    Shows(t, tag)
}

// Gets a new, equal, MutData each time, which it passes on as different.
@Composable
fun ShowsVia(m: MutData) {
    Runs.hit("ShowsVia")
    InlineShows(m, "via inline")
}

@Composable
inline fun InlineItems(tag: String, item: @Composable (String) -> Unit) {
    Runs.hit("InlineItems $tag")
    item(tag)
}

// Not composable: its lambda isn't remembered, also where it is copied.
inline fun clicker(s: String): () -> Unit = { println(s) }

// Passes its parameters on: the callees get its change flags for them.
@Composable
fun Forward(s: String, p: Point, m: Mutable, shape: Shape) {
    Runs.hit("Forward")
    Strings(s, 1)
    Points(p)
    Mutables(m)
    Shapes(shape)
    Counted("forward", s)
    s.Suffixed(2)
}

@Composable
fun <T> ForwardT(t: T) {
    Runs.hit("ForwardT")
    Anything(t, "forwarded")
}

// Reads state of its own: only it recomposes when it changes, and its
// default value isn't computed again.
@Composable
fun Reads(state: State<Int>, label: String = prefix()) {
    Runs.hit("Reads")
    Text("reads ${state.value} $label")
}

@Composable
fun HoldsState(state: MutableState<String>) {
    Runs.hit("Holds state")
    Text("holds ${state.value}")
}

@Composable
fun Skips(tick: Int, label: String, state: MutableState<String>, counter: State<Int>) {
    Runs.hit("Skips")
    Text("tick $tick")
    Strings(label, 1)
    Unused(label, tick)
    Mutables(remember { Mutable(1) })
    Mutables(Mutable(2))
    Points(Point(1, 2))
    Bags(Bag(listOf("a")), listOf("b"))
    Bases(Base(1), Derived(1, 2))
    Marks(Marked(1), Frozen(2))
    Modes(Mode.On, Meters(3), Origin)
    Holders(Holds("s"), Holds(Mutable(3)), Pair2("a", 1))
    Shapes(Circle("circle"))
    Shapes(Square("square"))
    Anything(label, "label")
    Anything(Mutable(4), "mutable")
    Anything(Point(0, 0), "point")
    val node = remember { Node("n") }
    Nodes(node, remember { Lazy2() })
    Panels(remember { Panel("kept") }, "kept")
    Panels(Panel("new"), "new")
    Defaulted()
    Defaulted("e")
    Varargs("a", label)
    Varargs()
    label.Suffixed(1)
    Card("c").Show(1)
    val card = remember { Card("kept") }
    card.Show(tick % 1)
    card.Plain(1)
    remember { Board("b") }.Show(1)
    Clicks("none") { println("none") }
    Clicks("label") { println(label) }
    Clicks("tick") { println(tick) }
    val mutable = remember { Mutable(5) }
    Clicks("mutable") { println(mutable.v) }
    Content("label") {
        Runs.hit("content label")
        Text("content $label")
    }
    Content("tick") {
        Runs.hit("content tick")
        Text("content $tick")
    }
    Items("items") { s ->
        Runs.hit("item $s")
        Text("item $s")
    }
    Box("box") {
        Runs.hit("box content")
        Strings(label, 2)
    }
    Always(label)
    Forward(label, Point(0, 0), mutable, Square("forwarded"))
    ForwardT(label)
    Counted("label", label)
    Counted("tick", tick)
    Reads(counter)
    HoldsState(state)
    listOf("x", "y").forEach { s -> Clicks("each $s") { println(s) } }
    Shows(MutData(1), "mutable")
    InlineShows(MutData(1), "inline mutable")
    InlineShows(label, "inline label")
    ShowsVia(MutData(2))
    InlineItems(label) { s -> Strings(s, 3) }
    Calls("remembered") { println(label) }
    Calls("dont", @DontMemoize { println(label) })
    Calls("copied", clicker(label))
}

fun noop() {}

class Card2(val title: String) {
    fun touch() {}

    // Calls its members on this, which it passes on, and remembers a
    // lambda that captures it.
    @Composable
    fun Both(n: Int) {
        Runs.hit("Card2")
        Show(n)
        Clicks("card2") { println(title) }
        Clicks("card2 this") { touch() }
    }

    @Composable
    fun Show(n: Int) {
        Runs.hit("Card2 show")
        Text("card2 $title $n")
    }
}

@Composable
fun String.Twice2(n: Int) {
    Runs.hit("Twice2")
    Suffixed(n)
    Clicks("receiver") { println(length) }
}

@Composable
fun Eleven(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int, h: Int, i: Int, j: Int, k: String) {
    Runs.hit("Eleven")
    Text("eleven $a $k")
    Strings(k, j)
}

// Not restartable: it can't skip, and passes on what its caller said of
// its parameters.
@Composable
fun Computes(s: String, n: Int): String {
    Runs.hit("Computes")
    Clicks("computes") { println(s) }
    Strings(s, n)
    return s + n
}

@Composable
fun Skips2(tick: Int, label: String) {
    Runs.hit("Skips2")
    Card2("c2").Both(1)
    remember { Card2("kept") }.Both(tick % 1)
    label.Twice2(1)
    Eleven(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, label)
    Eleven(1, 2, 3, 4, 5, 6, 7, 8, 9, tick, "k")
    Text(Computes(label, 1))
    val copy = label
    Clicks("copy") { println(copy) }
    val upper = label.uppercase()
    Clicks("upper") { println(upper) }
    Clicks("ref", ::noop)
    val card = remember { Card2("ref") }
    Clicks("bound", card::touch)
    Scoped { i ->
        Runs.hit("scoped $this")
        Text("scoped $this $i $label")
    }
    Locals(label)
    Content("nested") {
        Runs.hit("nested outer")
        Content("inner") {
            Runs.hit("nested inner")
            Text("nested $tick")
        }
    }
}

// Lambdas in a local function, which isn't composable, aren't
// remembered; a suspend lambda is.
@Composable
fun Locals(label: String) {
    Runs.hit("Locals")
    fun make(): () -> Unit = { println(label) }
    Clicks("local", make())
    LaunchedEffect(label) { Runs.hit("launched $label") }
    Clicks("in try") {
        try {
            println(label)
        } catch (e: Exception) {
            println(e)
        }
    }
}
