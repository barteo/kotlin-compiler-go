package app

import androidx.compose.runtime.Composable
import lib.BaseStyled
import lib.Box
import lib.Decorated
import lib.Runs
import lib.Styled
import lib.Text

// An open class whose composables have default values, which read the
// object and the extension receiver.
open class Badge(val name: String) {
    @Composable
    open fun Show(label: String, mark: String = name, times: Int = 2) {
        Runs.hit("badge $name")
        Text(label + mark.repeat(times))
    }

    @Composable
    open fun String.tagged(prefix: String = "$this:", suffix: String = name): String = prefix + this + suffix
}

class Loud(name: String) : Badge(name) {
    @Composable
    override fun Show(label: String, mark: String, times: Int) {
        Text("LOUD " + label + mark.repeat(times))
    }
}

// An interface whose composable has a default value, which kotlinc's
// code implements.
interface Tile {
    @Composable
    fun Draw(n: Int, unit: String = "px")
}

// The program's override of the library's open composable.
class Plain : BaseStyled() {
    @Composable
    override fun Note(text: String, n: Int) {
        Text("plain $text $n")
    }
}

@Composable
fun Badges(b: Badge, s: Styled, t: Tile, x: Int) {
    b.Show("a")
    b.Show("b", times = x)
    Loud("l").Show("c", "!")
    Text(with(b) { "s".tagged() })
    Text(with(b) { "s".tagged(suffix = "?") })
    s.Chip("chip")
    s.Chip("chip", suffix = "]")
    Text(with(s) { "d".decorated() })
    Plain().Note("p")
    BaseStyled().Note("q", 3)
    t.Draw(x)
}

// Lambdas converted to a fun interface whose method is composable: objects
// whose composable overrides have their bodies.
@Composable
fun Decorations(n: Int) {
    Decorated("plain", null)
    Decorated("boxed", decorator = { inner -> Box("deco $n") { inner() } })
    Decorated("twice") { inner ->
        Runs.hit("twice")
        inner()
        inner()
    }
}
