// An enum entry is stable, so are its vals: a check of one smart casts it,
// as an object's does.
import Message.WORLD

enum class Message(val text: String?, var note: String?) {
    HELLO("hello", null),
    WORLD("world", "n"),
    NOTHING(null, null),
}

fun main() {
    Message.HELLO.text!!
    println(Message.HELLO.text.length)
    if (Message.NOTHING.text != null) println(Message.NOTHING.text.length) else println("none")
    if (WORLD.text is String) println(WORLD.text.uppercase())
    val n = Message.WORLD.note
    if (n != null) println(n.length)
}
