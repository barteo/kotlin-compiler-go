import java.util.Nope // ERROR unresolved import: java.util.Nope
import java.lang.Math.nope // ERROR unresolved import: java.lang.Math.nope
import java.lang.String.length // ERROR cannot import 'length'. Functions and properties can only be imported from packages or objects
import nope.* // ERROR unresolved import: nope.*

fun main() {
    val list = java.util.ArrayList() // ERROR cannot infer type for type parameter 'E'
    val map: java.util.HashMap? = null // ERROR 2 type arguments expected for class HashMap<K, V>
    val stream = java.io.InputStream() // ERROR cannot create an instance of InputStream
    println(Math.max("a", 1)) // ERROR none of the 4 candidates for 'max' takes (String, Int)
    println(Math.nope(1)) // ERROR unresolved reference: nope
    val sb = StringBuilder()
    println(sb.length()) // ERROR cannot be invoked as a function
    sb.append(null) // ERROR ambiguous call
    println(StringBuilder.append("x")) // ERROR 'append' is not a static method of StringBuilder
    val m = Math // ERROR class 'Math' cannot be used as a value
    val a: Any = 1
    val s: String = a // ERROR inferred type is Any but String was expected
    Integer.MAX_VALUE = 3 // ERROR val cannot be reassigned
    System.out = System.err // ERROR val cannot be reassigned
    Thread.currentThread().name = "worker"
    Thread.currentThread().isAlive = false // ERROR val cannot be reassigned
    val cs: CharSequence = 1 // ERROR inferred type is Int but CharSequence was expected
}

// A Kotlin property doesn't override a Java getter: its getter clashes.
abstract class Named : java.security.Principal {
    val name: String = "x" // ERROR accidental override: the following declarations have the same JVM signature (getName()Ljava/lang/String;)
}

class NamedTwice : java.security.Principal {
    override val name: String = "x" // ERROR 'name' overrides nothing
    override fun getName() = "y" // ERROR platform declaration clash: the following declarations have the same JVM signature (getName()Ljava/lang/String;)
}
