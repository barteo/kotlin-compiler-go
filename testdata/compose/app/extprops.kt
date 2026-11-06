package app

import androidx.compose.runtime.Composable
import lib.Insets
import lib.Runs
import lib.Text
import lib.visible

// Composable extension properties: their getters take the receiver, then
// a Composer and change flags.
val Insets.Companion.shown: String
    @Composable get() = if (visible) "shown" else "hidden"

val Insets.Companion.shownTwice: String
    @Composable get() = shown + "/" + Insets.shown

val String.boxed: String
    @Composable get() {
        Runs.hit("boxed $this")
        Text("boxing $this")
        return "[$this]"
    }

val <T> List<T>.second: T
    @Composable get() = this[1]

@get:Composable
val Int.described: String
    get() = describe(this)

// A class whose composable extension property reads the object and the
// receiver: its composable uses the object.
class Scale(val k: Int) {
    val Int.scaled: Int
        @Composable get() = this * k

    @Composable
    fun Show(n: Int) {
        Text("scale ${n.scaled}")
    }
}

@Composable
fun ExtProps(label: String, n: Int) {
    Text(Insets.shownTwice)
    Text(label.boxed)
    Text("second ${listOf(label, "b").second}")
    Text(n.described)
    Scale(3).Show(n)
    with(Scale(n)) { Text("with ${2.scaled}") }
    Text("safe ${label.takeIf { n > 1 }?.boxed}")
}
