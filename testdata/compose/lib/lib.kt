package lib

import androidx.compose.runtime.AbstractApplier
import androidx.compose.runtime.BroadcastFrameClock
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ComposeNode
import androidx.compose.runtime.Composition
import androidx.compose.runtime.Recomposer
import androidx.compose.runtime.snapshots.Snapshot
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.coroutines.yield

// A node of the tree that a composition builds: a box, with a name and
// children, or a text.
class Node(val name: String) {
    var text: String? = null
    val children = mutableListOf<Node>()

    fun dump(out: StringBuilder, indent: String) {
        out.append(indent).append(name)
        text?.let { out.append(" \"").append(it).append('"') }
        out.append('\n')
        for (c in children) {
            c.dump(out, "$indent  ")
        }
    }
}

class NodeApplier(root: Node) : AbstractApplier<Node>(root) {
    override fun insertTopDown(index: Int, instance: Node) {}

    override fun insertBottomUp(index: Int, instance: Node) {
        current.children.add(index, instance)
    }

    override fun remove(index: Int, count: Int) {
        current.children.remove(index, count)
    }

    override fun move(from: Int, to: Int, count: Int) {
        current.children.move(from, to, count)
    }

    override fun onClear() {
        root.children.clear()
    }
}

@Composable
fun Text(text: String) {
    ComposeNode<Node, NodeApplier>(factory = { Node("text") }, update = { set(text) { this.text = it } })
}

@Composable
fun Box(name: String, content: @Composable () -> Unit) {
    ComposeNode<Node, NodeApplier>(factory = { Node(name) }, update = {}, content = content)
}

@Composable
inline fun Row(content: @Composable () -> Unit) {
    ComposeNode<Node, NodeApplier>(factory = { Node("row") }, update = {}, content = content)
}

// Calls item for each of items, in a group of their own, where keyed
// groups move when their order changes.
@Composable
inline fun <T> ForEach(items: List<T>, item: @Composable (T) -> Unit) {
    for (x in items) {
        item(x)
    }
}

@Composable
inline fun <R> Measure(block: @Composable () -> R): R = block()

// A library composable with default values, which a program's calls
// leave out.
@Composable
fun Label(text: String, prefix: String = "> ", suffix: String = "") {
    Text(prefix + text + suffix)
}

// Composable function types of every shape, which a program passes its
// lambdas to, and invokes.
@Composable
fun Scope(block: @Composable String.(Int) -> Unit) {
    "lib".block(7)
}

@Composable
fun Twice(item: @Composable (Int) -> Unit) {
    item(1)
    item(2)
}

fun label(s: String): @Composable () -> Unit = { Text("label $s") }

val Divider: @Composable () -> Unit = { Text("--") }

// A composable extension property of a companion, as Compose's
// WindowInsets.isImeVisible: its getter takes a Composer.
class Insets {
    companion object
}

val Insets.Companion.visible: Boolean
    @Composable get() = currentComposerName() == "composer"

@Composable
fun currentComposerName(): String = "composer"

// An interface whose composable a program's class implements.
interface Widget {
    @Composable
    fun Render(label: String)
}

// An interface and a class whose open composables have default values:
// callers call them through the static name$default methods of their
// ComposeDefaultImpls classes.
interface Styled {
    @Composable
    fun Chip(text: String, prefix: String = "[", suffix: String = text.take(1))

    @Composable
    fun String.decorated(times: Int = 2): String
}

open class BaseStyled : Styled {
    @Composable
    override fun Chip(text: String, prefix: String, suffix: String) {
        Text(prefix + text + suffix)
    }

    @Composable
    override fun String.decorated(times: Int): String = repeat(times)

    @Composable
    open fun Note(text: String, n: Int = count()) {
        Text("note $text $n")
    }

    open fun count() = 1
}

// A fun interface whose method is composable, as Compose's
// TextFieldDecorator.
fun interface Decorator {
    @Composable
    fun Decoration(inner: @Composable () -> Unit)
}

@Composable
fun Decorated(label: String, decorator: Decorator?) {
    if (decorator == null) Text(label) else decorator.Decoration { Text(label) }
}

// Runs counts the runs of the bodies of the composables that call hit,
// which drive prints after each composition: what recomposes, and what
// skips.
object Runs {
    private val counts = sortedMapOf<String, Int>()

    fun hit(name: String) {
        counts[name] = (counts[name] ?: 0) + 1
    }

    fun take(): String {
        val s = counts.entries.joinToString(" ") { "${it.key}=${it.value}" }
        counts.clear()
        return s
    }
}

// A composable of the library that counts its runs.
@Composable
fun Counted(name: String, value: Any?) {
    Runs.hit("lib $name")
    Text("$name $value")
}

// drive composes content and prints its tree and the runs of counted
// composables, then, for each step, runs it, which changes state,
// recomposes and prints them again. Last it disposes of the composition.
// Frames are sent only while the recomposer waits for one.
fun drive(steps: List<() -> Unit>, content: @Composable () -> Unit) = runBlocking {
    val clock = BroadcastFrameClock()
    withContext(clock) {
        val recomposer = Recomposer(coroutineContext)
        launch { recomposer.runRecomposeAndApplyChanges() }
        val root = Node("root")
        val composition = Composition(NodeApplier(root), recomposer)
        var frame = 0L
        suspend fun settle() {
            while (true) {
                yield()
                if (clock.hasAwaiters) {
                    clock.sendFrame(++frame * 16_000_000)
                } else if (!recomposer.hasPendingWork) {
                    return
                }
            }
        }
        fun print() {
            val out = StringBuilder()
            root.dump(out, "")
            out.append("runs: ").append(Runs.take()).append('\n')
            kotlin.io.print(out)
        }
        composition.setContent(content)
        settle()
        print()
        for (step in steps) {
            step()
            Snapshot.sendApplyNotifications()
            settle()
            print()
        }
        composition.dispose()
        recomposer.cancel()
    }
}

// A composable with default values and more than 31 parameters, which
// takes two ints of masks of the omitted arguments.
@Composable
fun WideLib(
    label: String,
    p0: Int = 0,
    p1: Int = 1,
    p2: Int = 2,
    p3: Int = 3,
    p4: Int = 4,
    p5: Int = 5,
    p6: Int = 6,
    p7: Int = 7,
    p8: Int = 8,
    p9: Int = 9,
    p10: Int = 10,
    p11: Int = 11,
    p12: Int = 12,
    p13: Int = 13,
    p14: Int = 14,
    p15: Int = 15,
    p16: Int = 16,
    p17: Int = 17,
    p18: Int = 18,
    p19: Int = 19,
    p20: Int = 20,
    p21: Int = 21,
    p22: Int = 22,
    p23: Int = 23,
    p24: Int = 24,
    p25: Int = 25,
    p26: Int = 26,
    p27: Int = 27,
    p28: Int = 28,
    p29: Int = 29,
    p30: Int = 30,
    p31: Int = 31,
    p32: Int = 32,
    last: String = "end",
) {
    Runs.hit("lib wide $label")
    Text("$label p0=$p0 p31=$p31 p32=$p32 sum=${listOf(p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14, p15, p16, p17, p18, p19, p20, p21, p22, p23, p24, p25, p26, p27, p28, p29, p30, p31, p32).sum()} $last")
}
