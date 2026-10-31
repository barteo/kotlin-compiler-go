import objs.*

fun main() {
    val local = "L"
    println(wrap { "a" + local }.run())
    val c = counter(5)
    c.run()
    println(c.run())
    println(Host("bob").greeter { "!" }.run())
    println(wrap { "b" }.run() + wrap { "c" }.run())
}
