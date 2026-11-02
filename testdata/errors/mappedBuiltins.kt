// Errors of JVM-mapped built-in members that a Java superclass doesn't
// implement.

class Partial : Number() { // ERROR is not abstract and does not implement abstract
    override fun toInt(): Int = 0
    override fun toLong(): Long = 0
    override fun toFloat(): Float = 0f
    override fun toDouble(): Double = 0.0
}

class Atomic : java.util.concurrent.atomic.AtomicInteger(1) // ERROR is not abstract and does not implement abstract

interface Sized { val size: String }
class Names : java.util.ArrayList<String>(), Sized // ERROR is not abstract and does not implement abstract

