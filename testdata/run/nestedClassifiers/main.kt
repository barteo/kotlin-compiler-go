import other.Mode

// An interface's nested classes aren't in scope in the classes that
// implement it: Mode is the imported one in Screen.
interface Widget {
    enum class Mode { Disabled, Enabled }
}

data class Screen(val mode: Mode) : Widget {
    fun next(): Mode = if (mode == Mode.Shown) Mode.Hidden else Mode.Shown
}

// A superclass's are.
open class Base {
    enum class Kind { A, B }
}

class Derived(val kind: Kind) : Base() {
    fun other(): Kind = if (kind == Kind.A) Kind.B else Kind.A
}

// A class's supertypes don't see its own nested classes: Reply
// implements the outer Page, and its property is the nested one.
sealed interface Page {
    data class Reply(val title: String, val page: Page) : Page {
        data class Page(val count: Int)
    }
}

fun describe(p: Page): String = when (p) {
    is Page.Reply -> p.title + " " + p.page.count
}

fun main() {
    println(Screen(Mode.Shown).next())
    println(Widget.Mode.Enabled)
    println(Derived(Base.Kind.A).other())
    println(describe(Page.Reply("pages", Page.Reply.Page(3))))
}
