// Char: literals, arithmetic, comparisons, ranges, strings and CharArray.

fun classify(c: Char): String = when (c) {
    'a', 'e', 'i', 'o', 'u' -> "vowel"
    in 'a'..'z' -> "consonant"
    in '0'..'9' -> "digit"
    ' ' -> "space"
    else -> "other"
}

fun caesar(s: String, shift: Int): String {
    var out = ""
    for (c in s) {
        out += if (c in 'a'..'z') 'a' + (c - 'a' + shift) % 26 else c
    }
    return out
}

fun count(s: String, target: Char): Int {
    var n = 0
    for (i in 0 until s.length) {
        if (s[i] == target) n++
    }
    return n
}

class Cell(var mark: Char)

fun main() {
    val a = 'a'
    val newline = '\n'
    val quote = '\''
    val backslash = '\\'
    val unicode = 'A'
    val e = 'é'
    println(a)
    println("[" + newline + "]")
    println("$quote$backslash$unicode$e")
    println(a.code)
    println(unicode.code + 1)
    println(Char(98))
    println(99.toChar())
    println(a + 2)
    println('z' - 1)
    println('z' - 'a')
    println(a < 'b')
    println('Z' >= 'a')
    println(a == 'a')
    println(a != 'b')
    val nullable: Char? = if (a.code > 0) 'q' else null
    println(nullable)
    println(nullable == 'q')
    var c = 'x'
    c++
    println(c)
    c--
    c += 2
    println(c)
    c -= 1
    println(c)

    for (ch in 'a'..'e') print(ch)
    println()
    for (ch in 'e' downTo 'a' step 2) print(ch)
    println()
    for (ch in 'a' until 'd') print(ch)
    println()
    println('m' in 'a'..'z')
    println('M' !in 'a'..'z')
    for (s in charArrayOf('a', 'b', '7', ' ', '#')) print(classify(s) + " ")
    println()
    println(caesar("hello, world", 3))
    println(count("banana", 'a'))
    println("kotlin"[2])
    println("kotlin".get(0))

    val chars = CharArray(3)
    chars[0] = 'x'
    chars[1] = 'y'
    chars[2] = chars[0] + 2
    println(String(chars))
    val fromString = "abc".toCharArray()
    fromString[1]++
    println(String(fromString))
    val letters = charArrayOf('k', 'g', 'o')
    for (l in letters) print(l)
    println()
    println(letters.size)

    println(Character.isDigit('7'))
    println(Character.toUpperCase('q'))
    println('7'.isDigit())
    println('x'.isLetter())
    println('X'.isUpperCase())
    println('x'.uppercaseChar())
    println('Q'.lowercaseChar())
    println('_'.isJavaIdentifierStart())
    println('-'.isJavaIdentifierPart())
    val sb = StringBuilder()
    sb.append('h').append('i')
    println(sb)
    println(sb.toString()[1])
    val cell = Cell('o')
    cell.mark = 'x'
    println("mark: ${cell.mark}")
    var captured = 'a'
    val bump = { captured++ }
    bump()
    bump()
    println(captured)
    val boxed: Any = 'w'
    if (boxed is Char) println(boxed.code)
    println(boxed as Char)
    println(java.lang.String.valueOf('v'))
}
