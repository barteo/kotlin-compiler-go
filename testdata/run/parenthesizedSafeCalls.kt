// Parentheses end a safe call's chain: (a?.b)[i] indexes a nullable value,
// so the operators of a nullable receiver's extensions apply.
var result = "none"

class Foo {
    var alias: Foo = this
    operator fun get(index: Int): Foo { result = "get: member"; return this }
    operator fun set(index: Int, arg: Foo) { result = "set: member" }
    operator fun plusAssign(arg: String) { result = "plusAssign: member" }
    operator fun inc(): Foo { result = "inc: member"; return this }
    operator fun invoke(arg: String) { result = "invoke: member" }
}

operator fun Foo?.get(index: Int): Foo? { result = "get: extension"; return this }
operator fun Foo?.set(index: Int, arg: Foo?) { result = "set: extension" }
operator fun Foo?.plusAssign(arg: String) { result = "plusAssign: extension" }
operator fun Foo?.inc(): Foo { result = "inc: extension"; return this ?: Foo() }
operator fun Foo?.invoke(arg: String) { result = "invoke: extension" }

fun show(what: String) {
    println("$what => $result")
    result = "none"
}

fun huh(arg: Foo?, foo: Foo) {
    arg?.alias[42] = arg; show("arg?.alias[42] = arg")
    arg?.alias[42] += ""; show("arg?.alias[42] += \"\"")
    arg?.alias[42]++; show("arg?.alias[42]++")
    ++arg?.alias[42]; show("++arg?.alias[42]")
    (arg?.alias)[42]; show("(arg?.alias)[42]")
    (arg?.alias)[42] = arg; show("(arg?.alias)[42] = arg")
    (arg?.alias) += ""; show("(arg?.alias) += \"\"")
    (arg?.alias)(""); show("(arg?.alias)(\"\")")
    (arg?.alias)[42] += ""; show("(arg?.alias)[42] += \"\"")
    (arg?.alias[42]) += ""; show("(arg?.alias[42]) += \"\"")
    (arg?.alias[42])++; show("(arg?.alias[42])++")
    ++(arg?.alias[42]); show("++(arg?.alias[42])")
    ++foo[1]; show("++foo[1]")
    (foo.alias) += ""; show("(foo.alias) += \"\"")
}

fun main() {
    huh(Foo(), Foo())
    huh(null, Foo())
}
