import java.util.Calendar
import java.util.Calendar.FEBRUARY
import java.util.jar.JarFile

// Java constants (static final fields with a ConstantValue) are inlined,
// as kotlinc and javac inline them: code compiled against one version of
// a class runs with another that lacks the field, as Android's are.
fun main() {
    println(java.lang.Integer.MAX_VALUE)
    println(java.lang.Long.MIN_VALUE)
    println(java.lang.Byte.SIZE)
    println(Character.MAX_RADIX)
    println(Calendar.JANUARY + FEBRUARY)
    println(Math.PI)
    println(java.lang.Float.MAX_VALUE)
    println(java.lang.Float.NaN != java.lang.Float.NaN)
    println(java.lang.Double.POSITIVE_INFINITY)
    println(JarFile.MANIFEST_NAME)
    println(Character.MIN_SUPPLEMENTARY_CODE_POINT)
    println(java.lang.Short.MAX_VALUE + 1)
    println(if (java.lang.Character.MIN_VALUE == '\u0000') "min" else "?")
    val sdk = 33
    if (sdk >= java.sql.Types.BIGINT) println("big") // -5
}
