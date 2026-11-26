// An enum entry's var isn't stable: a check of it doesn't smart cast it.
enum class Message(var note: String?) { HELLO(null) }

fun f(): Int {
    if (Message.HELLO.note != null) return Message.HELLO.note.length // ERROR nullable receiver
    return 0
}
