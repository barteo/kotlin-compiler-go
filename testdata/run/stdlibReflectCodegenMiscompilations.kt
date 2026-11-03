// A companion's private properties, which kotlin-reflect reads.
import kotlin.reflect.jvm.isAccessible

@JvmInline
value class S(val s: String)

class Host {
    companion object {
        private val ok = S("OK")
        private val n = 42
        private var v = S("a")
        val refOk = ::ok.apply { isAccessible = true }
        val refN = ::n.apply { isAccessible = true }
        val refV = ::v.apply { isAccessible = true }
    }
}

fun main() {
    println(Host.refOk.call().s)
    println(Host.refN.get())
    Host.refV.set(S("b"))
    println(Host.refV.get().s + Host.refV.getter.call().s)
}
