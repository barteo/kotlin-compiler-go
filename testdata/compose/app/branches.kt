package app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import lib.Text

fun label(make: (() -> String)?): String = make?.invoke() ?: "none"

fun keep(make: () -> String): () -> String = make

// A lambda that a branch remembers, in the branch's group: the values
// remembered after it stay where they are when the other branch runs.
@Composable
fun Branches(on: Boolean, n: Int) {
    val first = label(if (on) keep { "on" } else null)
    val state = remember { mutableStateOf("state") }
    val later = keep { "later $n" }
    val second = when {
        on -> "none"
        else -> keep { "off $n" }()
    }
    val third = on || keep { "or" }() == "or"
    val kept = remember { mutableStateOf("kept") }
    Text("branches $first ${state.value} ${later()} $second $third ${kept.value}")
}
