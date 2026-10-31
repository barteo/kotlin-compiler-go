package app

import androidx.compose.runtime.Composable
import lib.Box
import lib.Text

// Composable lambdas kept for later.
class Slots {
    val items = mutableListOf<@Composable (String) -> Unit>()

    fun add(content: @Composable (String) -> Unit) {
        items += content
    }
}

// Public inline functions whose composable lambdas are objects of classes
// of their own, which callers copy with their bodies: with the call's
// type argument for a reified one.
inline fun <reified T : Any> Slots.typed(noinline content: @Composable (String) -> Unit) {
    add { s -> Box("typed") { content(s + " " + T::class.simpleName) } }
}

inline fun Slots.plain(label: String) {
    add { s -> Text("$label $s") }
    add { Text("fixed $it") }
}

@Composable
fun ShowSlots(slots: Slots) {
    for (c in slots.items) {
        c("slot")
    }
}

// The program's copies.
fun appSlots(): Slots = Slots().apply {
    typed<String> { Text("app typed $it") }
    plain("app")
}
