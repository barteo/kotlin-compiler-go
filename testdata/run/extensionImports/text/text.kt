package text

fun String.words(): Int {
    var n = 0
    var inWord = false
    for (c in this) {
        if (c == ' ') {
            inWord = false
        } else if (!inWord) {
            inWord = true
            n++
        }
    }
    return n
}

fun String.tag(): String = "text:" + this
fun Int.tag(): String = "text-int:$this"

val String.firstWord: String
    get() {
        var end = 0
        while (end < length && this[end] != ' ') end++
        return substring(0, end)
    }
