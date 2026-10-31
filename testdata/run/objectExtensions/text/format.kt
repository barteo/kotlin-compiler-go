package text

class Formatter(val prefix: String) {
    fun format(s: String) = prefix + s.shout + " " + s.twice()

    companion object {
        val String.shout: String get() = this + "!"
        fun String.twice() = this + this
        val Char.isVowel: Boolean
            get() = this == 'a' || this == 'e' || this == 'i' || this == 'o' || this == 'u'

        fun vowels(s: String): Int {
            var n = 0
            for (c in s) {
                if (c.isVowel) n++
            }
            return n
        }
    }
}

object Strings {
    val String.initial: Char get() = this[0]
    fun String.wrap(c: Char) = "" + c + this + c
}

class Counter(val step: Int) {
    val Int.next: Int get() = this + step
    fun from(n: Int) = n.next.next
}
