import lib.*

fun main() {
    var flag = true
    println((Stub<Boolean>() answers { flag }).how)
    println((Stub<String>() answers { "$call/$it" }).how)
    println((Stub<String>() answers object : Answer<String> {
        override fun answer(call: String) = "obj $call"
    }).how)
    println(countIf(listOf(1, 2, 3, 4)) { it % 2 == 0 })
    var n = 0
    runTwice { n++ }
    println(n)
    println(callIt { "called" })
}
