package app

import androidx.compose.runtime.Composable
import lib.Text

interface Area {
    val width: Int

    fun describe(): String
}

val Area.half: Int get() = width / 2

fun Area.doubled(): Int = width * 2

@Composable
fun WithArea(width: Int, content: @Composable Area.() -> Unit) {
    val area = object : Area {
        override val width: Int = width

        override fun describe(): String = "area $width"
    }
    area.content()
}

// Composable lambdas that read their receiver without naming it: they
// run again for another receiver.
@Composable
fun Areas(n: Int) {
    WithArea(n) { Text("width $width") }
    WithArea(n) { Text(describe()) }
    WithArea(n) { Text("half $half") }
    WithArea(n) { Text("doubled ${doubled()}") }
    WithArea(n) { Text("this ${this.width}") }
    WithArea(n) {
        val wide = width > 3
        Text(if (wide) "wide" else "narrow")
    }
}
