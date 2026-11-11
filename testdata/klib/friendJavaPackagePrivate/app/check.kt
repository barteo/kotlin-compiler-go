// friend
package cmd

fun check() {
    val c = Command()
    c.zipped = true
    println(c.zipped)
    println(c.idString())
    println(Command.parseId(" 42 "))
    println(describe(c))
    println(Command.RELATION + " " + Command.counter + " " + Command.tag("t"))
}
