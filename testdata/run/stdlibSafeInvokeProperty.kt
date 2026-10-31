// x?.f(args), where f is a property of function type, invokes it only if
// x isn't null, and evaluates the arguments only then; x.f().not() is a
// condition that smart-casts, as !x.f() does.
class Holder(val content: (Boolean, String) -> String)

class Scope(var holder: Holder?)

fun arg(s: String): String {
    println("arg $s")
    return s
}

fun describe(s: String?) {
    if (s.isNullOrBlank().not()) {
        println("not blank: ${s.length}")
    } else {
        println("blank")
    }
}

fun main() {
    val s = Scope(Holder { b, t -> "$b $t" })
    println(s.holder?.content(true, arg("x")))
    s.holder = null
    println(s.holder?.content(false, arg("y")))
    val r: String = s.holder?.content(true, "z") ?: "none"
    println(r)
    describe("abc")
    describe(" ")
    describe(null)
}
