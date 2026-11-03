// DSL markers: an implicit receiver or context argument can't be used
// where another with the same marker is nearer.
@DslMarker
annotation class Html

@DslMarker
annotation class Css

@Html
interface Tag

class Body : Tag {
    var id = ""
    fun p(block: P.() -> Unit) = P().block()
    fun Tag.attr() {}
}

class P : Tag {
    fun text(s: String) {}
}

@Css
class Style {
    fun color() {}
}

fun Body.br() {}
var Body.title: String
    get() = ""
    set(value) {}

fun body(block: Body.() -> Unit) = Body().block()
fun style(block: Style.() -> Unit) = Style().block()

fun page() {
    body {
        id = "top"
        p {
            text("x")
            p {} // ERROR 'p' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            br() // ERROR 'br' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            id = "x" // ERROR 'id' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            title += "!" // ERROR 'title' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            println(id.length) // ERROR 'id' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            this@body.p {}
            this@body.br()
            style {
                color()
                text("y") // P's marker isn't Style's
            }
        }
        style {
            id = "other markers"
        }
    }
}

fun <T : Tag> generic(t: T, block: T.() -> Unit) = t.block()

fun inGeneric() {
    body {
        generic(P()) {
            br() // ERROR 'br' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
        }
    }
}

// The class's this, and a member extension's dispatch receiver.
class Page : Tag {
    fun head() {}
    fun build() {
        head()
        body {
            head() // ERROR 'head' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            this@Page.head()
        }
    }
}

fun Body.attrs() {
    p {
        attr() // ERROR 'attr' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
    }
}

// Context arguments, and other context parameters of the same level.
context(_: Body)
fun section() {}

context(_: Style)
fun rule() {}

context(_: Body)
val heading: String
    get() = ""

fun <T, R> context(value: T, block: context(T) () -> R): R = block(value)

fun <A, B, R> contexts(a: A, b: B, block: context(A, B) () -> R): R = block(a, b)

val sectionValue: context(Body) () -> Unit = {}

fun contexts() {
    body {
        section()
        p {
            section() // ERROR 'section' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            println(heading) // ERROR 'heading' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
            sectionValue() // ERROR 'sectionValue' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
        }
    }
    context(Body()) {
        context(Style()) {
            section()
            rule()
        }
        context(P()) {
            section() // ERROR 'section' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
        }
    }
    contexts(Body(), P()) {
        section() // ERROR 'section' cannot be called in this context with an implicit receiver: use an explicit receiver if necessary
    }
    contexts(Body(), Style()) {
        section()
        rule()
    }
}
