import java.math.BigInteger
import java.util.*
import java.util.concurrent.atomic.AtomicLong as Counter

class Holder(val sb: StringBuilder, var hits: Int)

fun describe(x: Any): String = "value " + x.toString()

fun main() {
    // Overloads: append(String), append(Int), append(Long), append(Double), append(Boolean).
    val sb = StringBuilder()
    sb.append("a").append(1).append(2L).append(2.5).append(true)
    println(sb.toString())
    println(sb.length)
    val copy = StringBuilder("x")
    copy.append(sb) // append(CharSequence)
    println(copy)

    // Static methods and fields.
    println(Math.max(1, 2))
    println(Math.max(1L, 5L))
    println(Math.abs(-7))
    println(Math.sqrt(16.0))
    println(Math.floorMod(-7, 3))
    println(Integer.MAX_VALUE)
    println(java.lang.Long.MIN_VALUE)
    println(java.lang.Double.compare(1.0, 2.0))
    println(Integer.parseInt("123") + 1)
    println(Integer.toHexString(255))
    System.out.println("from System.out")
    System.out.println(42)

    // Imports: star, qualified name, alias.
    val r = Random(42)
    println(r.nextInt(100))
    println(r.nextInt(100))
    val r2 = java.util.Random(7L)
    println(r2.nextLong() % 1000)
    val counter = Counter(3L)
    println(counter.incrementAndGet())
    val big = BigInteger.valueOf(2).pow(100)
    println(big)
    println(big.toString().length)
    println(big.bitLength())

    // Getters and setters as properties.
    val f = java.io.File("dir/sub/name.txt")
    println(f.name)
    println(f.parent)
    println(f.isAbsolute)
    val d = Date(0L)
    d.time = 1000L
    d.time += 500L
    println(d.time)
    println(Thread.currentThread().name)

    // Subtyping, Any and platform types.
    val cs: CharSequence = "hello"
    println(cs.length)
    println(describe(sb))
    println(describe(3))
    val any: Any = sb
    println(any == sb)
    println(any.hashCode() == sb.hashCode())
    val prop: String? = System.getProperty("kc.missing.property")
    println(prop ?: "no property")
    val dflt = System.getProperty("kc.missing.property", "default")
    println(dflt.length)
    val boxed = Integer.valueOf(41)
    println(boxed + 1)

    // Interfaces, safe calls, Java types in user classes.
    val bits = BitSet()
    bits.set(3)
    bits.set(5)
    println(bits.cardinality())
    println(bits.get(3))
    println(bits)
    println(bits.isEmpty)
    val maybe: StringBuilder? = null
    println(maybe?.length)
    println(maybe?.append("x"))
    val holder = Holder(StringBuilder("h"), 0)
    holder.sb.append("!")
    holder.hits++
    println("${holder.sb} ${holder.hits}")
    val atom = java.util.concurrent.atomic.AtomicInteger(5)
    println(atom.incrementAndGet())
    atom.set(10)
    println(atom.get())
}
