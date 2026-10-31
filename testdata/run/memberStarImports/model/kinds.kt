package model

enum class Overflow { SUSPEND, DROP_OLDEST, DROP_LATEST }

internal enum class Outcome { SUCCESSFUL, REREGISTER, CANCELLED }

class Shape {
    class Box(val side: Int)
    enum class Kind { ROUND, SQUARE }
}

// Private to this file: a star import of Overflow's entries in another
// file of the package isn't shadowed by it.
private val SUSPEND = "file-private"

fun privateSuspend() = SUSPEND
