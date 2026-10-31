import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import app.Badge
import app.Badges
import app.Counter
import app.Decorations
import app.ShowSlots
import app.Slots
import app.Tile
import app.appSlots
import app.plain
import app.typed
import app.DrawAll
import app.Both
import app.Compute
import app.Fancy
import app.Frame
import app.Header
import app.Holder
import app.Scoped
import app.footer
import app.Greeting
import app.Item
import app.Layout
import app.Pick
import app.Wrap
import app.Many
import app.Panel
import app.Screen
import app.Shout
import app.Tracked
import app.Until
import app.Content
import app.Skips
import app.Skips2
import app.Varargs
import app.Strings
import lib.Runs
import app.describe
import app.title
import lib.Text
import lib.Widget
import lib.drive

// A class that implements the program's interface, whose composable the
// program calls.
class Numbered(val prefix: String) : Item {
    @Composable
    override fun Draw(n: Int) {
        Text("$prefix$n")
    }
}

// A class that implements the program's interface, whose composable has
// a default value.
class Pixels : Tile {
    @Composable
    override fun Draw(n: Int, unit: String) {
        Text("$n$unit")
    }
}

// Composes the program's composables, from a composition that kotlinc
// compiles, and changes the state they read.
fun main() {
    val name = mutableStateOf("world")
    val n = mutableIntStateOf(1)
    val count = mutableIntStateOf(0)
    val shown = mutableStateOf(true)
    val widget: Widget = Fancy()
    val tick = mutableIntStateOf(0)
    val label = mutableStateOf("L")
    val text = mutableStateOf("T")
    val counter = mutableIntStateOf(0)
    val slots = Slots().apply {
        typed<Int> { Text("use typed $it") }
        plain("use")
    }
    val fromApp = appSlots()
    drive(
        listOf(
            { println("-- rename"); name.value = "kc" },
            { println("-- count"); count.intValue++ },
            { println("-- n"); n.intValue = 5 },
            { println("-- hide"); shown.value = false },
            { println("-- tick"); tick.intValue++ },
            { println("-- label"); label.value = "M" },
            { println("-- text"); text.value = "U" },
            { println("-- counter"); counter.intValue++ },
            { println("-- nothing") },
        ),
    ) {
        Screen(name.value, n.intValue)
        Counter(count)
        Greeting("direct")
        Many(1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)
        "use".Shout()
        Text(describe(7) + " " + title)
        Panel("use").Show(suffix = "?")
        widget.Render("as widget")
        DrawAll(Numbered("a"), Numbered("b"))
        if (shown.value) {
            Tracked("shown")
        }
        // The program's composables call kotlinc's lambdas, and kotlinc's
        // code the program's.
        Frame("use") { Text("use's lambda ${n.intValue}") }
        Both("x", "y") { Text("use's item $it") }
        Scoped { i -> Text("use's $this $i") }
        Text(Compute { "use computed $it" })
        Header()
        footer(9)()
        Holder { Text("use's holder") }.content()
        // kotlinc copies the program's inline composables.
        Wrap("use") { Text("use's wrapped ${n.intValue}") }
        Text(Pick(n.intValue, "~") { "use picked $it" })
        Text(Pick(7) { "use's default" })
        Layout.Column("use") { Text("use's column") }
        Until(n.intValue) { Text("use's until $it") }
        // kotlinc's code calls the program's skippable composables, with
        // its lambdas.
        Skips(tick.intValue, label.value, text, counter)
        Skips2(tick.intValue, label.value)
        Varargs("use", label.value)
        Strings(label.value, 3)
        Content("use") {
            Runs.hit("use content")
            Text("use content ${label.value}")
        }
        // The program's open composables with default values.
        Badges(Badge("B"), lib.BaseStyled(), Pixels(), n.intValue)
        Badge("U").Show("use")
        with(Badge("W")) { Text("w".tagged()) }
        // The program's inline functions' composable lambdas.
        ShowSlots(slots)
        ShowSlots(fromApp)
        Decorations(n.intValue)
    }
}
