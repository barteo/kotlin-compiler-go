// @JvmStatic on one accessor of a property of an object or a companion
// object: that accessor alone is a static method, which Java callers
// call on the class, here by reflection.

import java.lang.reflect.Modifier

class Counter {
    companion object {
        var x: Int = 1
            @JvmStatic set
        @set:JvmStatic var y: String = "y"
        @get:JvmStatic var g: Int = 2
        var doubled: Int = 3
            @JvmStatic get() = field * 2
        var plusOne: Int = 4
            @JvmStatic set(v) { field = v + 1 }
        @get:JvmStatic val computed: Int get() = x + g
        @get:JvmStatic @set:JvmStatic var both: Int = 5
        @JvmStatic var whole: Int = 6
        var plain: Int = 7
    }

    fun bump() { x += 10; g += 10; plusOne = plusOne }
}

object Registry {
    var x: Int = 1
        @JvmStatic set
    @get:JvmStatic var g: Int = 2
    var plusOne: Int = 3
        @JvmStatic set(v) { field = v + 1 }
    var doubled: Int = 4
        @JvmStatic get() = field * 2
    @JvmStatic var whole: Int = 5

    fun bump() { x += 10; g += 10; plusOne += 10; doubled += 1 }
}

interface Host {
    companion object {
        @get:JvmStatic val g: Int = 8
        @set:JvmStatic var s: Int = 9
    }
}

fun statics(c: Class<*>) {
    for (m in c.declaredMethods.filter { Modifier.isStatic(it.modifiers) && !it.isSynthetic }.sortedBy { it.name }) {
        println(c.name + "." + m.name + ": " + Modifier.toString(m.modifiers))
    }
}

fun main() {
    statics(Counter::class.java)
    statics(Registry::class.java)
    statics(Host::class.java)

    val c = Counter::class.java
    c.getMethod("setX", Int::class.java).invoke(null, 20)
    c.getMethod("setY", String::class.java).invoke(null, "z")
    c.getMethod("setPlusOne", Int::class.java).invoke(null, 30)
    c.getMethod("setBoth", Int::class.java).invoke(null, 40)
    println("" + Counter.x + " " + Counter.y + " " + Counter.plusOne + " " + Counter.both)
    println("" + c.getMethod("getG").invoke(null) + " " + c.getMethod("getDoubled").invoke(null) + " " +
        c.getMethod("getComputed").invoke(null) + " " + c.getMethod("getBoth").invoke(null))
    Counter().bump()
    Counter.x = 5
    Counter.y += "!"
    Counter.g++
    Counter.doubled += 1
    println("" + Counter.x + " " + Counter.y + " " + Counter.g + " " + Counter.doubled + " " + Counter.plusOne + " " + Counter.computed)

    val r = Registry::class.java
    r.getMethod("setX", Int::class.java).invoke(null, 50)
    r.getMethod("setPlusOne", Int::class.java).invoke(null, 60)
    println("" + Registry.x + " " + Registry.plusOne + " " + r.getMethod("getG").invoke(null) + " " + r.getMethod("getDoubled").invoke(null))
    Registry.bump()
    Registry.x++
    Registry.g = Registry.x + Registry.doubled
    Registry.whole += Registry.g
    println("" + Registry.x + " " + Registry.g + " " + Registry.plusOne + " " + Registry.doubled + " " + Registry.whole)

    val h = Host::class.java
    h.getMethod("setS", Int::class.java).invoke(null, 70)
    println("" + h.getMethod("getG").invoke(null) + " " + Host.s + " " + Host.g)
}
