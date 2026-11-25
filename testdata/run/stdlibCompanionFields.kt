// A companion object's properties, whose private static fields the outer
// class holds: the companion, nested classes, lambdas and inline functions
// read and write them through the outer class's access$getX$cp and
// access$setX$cp, and the outer class directly.

import java.lang.reflect.Modifier

class Store {
    companion object {
        val names: List<String> = listOf("a", "b")
        var count: Int = 0
        val isReady: Boolean = true
        private var secret: Long = 10L
        private val label = "store"
        var doubled: Int = 1
            get() = field * 2
            set(v) { field = v + 1 }
        val lazyName: String by lazy { "lazy-" + label }
        lateinit var late: String
        private lateinit var privateLate: String
        lateinit var privateSet: String
            private set
        @JvmField var shared: String = "shared"

        fun bump(): Long {
            count++
            secret += 5
            privateLate = "set"
            privateSet = "private set"
            return secret
        }

        fun reader(): () -> String = { "$label:$secret:$privateLate" }

        inline fun inlineRead(): Int = count + names.size
    }

    class Nested {
        fun peek() = "$count/$isReady/${names.first()}"
        fun poke() {
            count += 10
            doubled = 5
        }
    }

    fun read() = "$count $secret $label $doubled $lazyName $isReady"

    fun write() {
        count = 7
        secret *= 2
        doubled = 3
        late = "late"
        shared = "outer"
    }
}

fun main() {
    println(Store.bump())
    println(Store().read())
    Store().write()
    println(Store().read())
    println(Store.reader()())
    val n = Store.Nested()
    println(n.peek())
    n.poke()
    println(n.peek())
    println(Store.doubled)
    println(Store.inlineRead())
    println(Store.late + " " + Store.shared + " " + Store.privateSet)
    Store.count -= 1
    Store.doubled += 1
    println("${Store.count} ${Store.doubled}")
    for (name in listOf("names", "count", "isReady", "secret", "label", "doubled", "lazyName\$delegate", "late", "privateLate", "privateSet", "shared")) {
        val f = Store::class.java.getDeclaredField(name)
        println("$name ${Modifier.toString(f.modifiers)}")
    }
    println(Store::class.java.declaredMethods.map { it.name }.filter { it.startsWith("access$") }.sorted())
}
