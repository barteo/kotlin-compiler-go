// A value smart cast to a built-in type and another type, String & Int
// or A & String, has the built-in type's members and properties wherever
// it is in the intersection, as with kotlinc. Such a value can't exist,
// as String and Int are final, so the branches never run, but the code
// in them loads and verifies.
interface A { fun aaa(): String }

fun stringAndInt(x: Any?): String {
    if (x is String && x is Int) return "" + x.length + x.uppercase() + x.inc()
    return "not both"
}

fun intAndString(x: Any?): String {
    if (x is Int && x is String) return "" + x.length + x.inc()
    return "not both"
}

fun interfaceAndString(x: Any?): String {
    if (x is A && x is String) return x.aaa() + x.length + x.get(0)
    if (x is A) return x.aaa()
    return "neither"
}

fun charAndInt(x: Any?): String {
    if (x is Int && x is Char) return "" + x.code + x.inc()
    return "not both"
}

fun main() {
    println(stringAndInt("abc"))
    println(intAndString(1))
    println(interfaceAndString(object : A { override fun aaa() = "A" }))
    println(interfaceAndString("s"))
    println(charAndInt('c'))
}
