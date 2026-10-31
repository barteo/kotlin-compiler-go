// Inline-only functions of the standard library whose bodies kc copies
// into the caller.

import java.io.File
import java.nio.charset.StandardCharsets

fun describe(s: String): String = "[" + s + "]"

// An inlined body with branches in a superclass constructor's argument,
// before the object is initialized.
class Checked(s: String) : Exception(s.isNotBlank().toString())

fun firstCode(s: String): Int = if (s.isNotEmpty()) s.codePointAt(0) else -1

fun main() {
    val s = "héllo"
    println(s.isNotEmpty())
    println("".isNotEmpty())
    println("  ".isNotBlank())
    // As call arguments, with values on the stack below them.
    println(describe("x" + s.isNotEmpty()) + s.isNotBlank())
    println(1 + s.codePointAt(1) * 2)
    println(firstCode(s) + firstCode(""))
    // In a constructor's arguments, with the new object on the stack.
    println(StringBuilder("y" + "".isNotBlank()).append(s.isNotEmpty()))
    println(Checked(" ").message + " " + Checked("z").message)

    val r = "h.l+o".toRegex()
    println(r.matches(s) || "a1".matches("[a-z][0-9]".toRegex()))
    println("x".matches(r))
    println("%d-%s".format(42, "x") + "%05.1f".format(3.14159))
    println("12.50".toBigDecimal().add("0.25".toBigDecimal()))
    println(7.toBigDecimal())

    // Default arguments go through the name$default method's body.
    val f = File.createTempFile("kc-", ".txt")
    f.deleteOnExit()
    val w = f.bufferedWriter()
    w.write("line")
    w.close()
    println(f.readText())
    val o = f.outputStream()
    o.write(65)
    o.close()
    println(f.readText())
    val w2 = f.bufferedWriter(StandardCharsets.UTF_16)
    w2.write("é")
    w2.close()
    println(f.length())
    val w3 = f.bufferedWriter(bufferSize = 16)
    w3.write("abc")
    w3.close()
    println(f.length())
}
