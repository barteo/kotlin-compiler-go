// A class's companion's properties have their fields in the class, where
// kotlin-reflect looks for them, but where the class has a property of
// the same name.
import java.lang.reflect.Modifier
import kotlin.reflect.full.declaredMemberProperties
import kotlin.reflect.jvm.isAccessible

class A {
    companion object {
        private val secret = "s"
        val pub = 1
        var counter = 0
        lateinit var late: String
    }
}

class B {
    val x = "instance"

    companion object {
        val x = "companion"
    }
}

interface I {
    companion object {
        val y = "i"
    }
}

@JvmInline
value class Mode private constructor(val bits: Int) {
    companion object {
        val A = Mode(1)
    }
}

fun staticIn(c: Class<*>, name: String) =
    c.declaredFields.any { it.name == name && Modifier.isStatic(it.modifiers) }

fun main() {
    val p = A.Companion::class.declaredMemberProperties.first { it.name == "secret" }
    p.isAccessible = true
    println(p.call(A.Companion))
    A.counter++
    A.counter += 2
    println(A.counter)
    A.late = "l"
    println(A.late)
    println(A.Companion::pub.get())
    println(B().x + " " + B.x)
    println(I.y)
    println(Mode.A.bits)
    println("${staticIn(A::class.java, "pub")} ${staticIn(A::class.java, "secret")}")
}
