// kotlinc: -jvm-default=enable
import lib.*
import java.lang.reflect.Modifier

// The program's classes, compiled with -jvm-default=enable too, get
// bridges to the default methods of the library's interfaces.
class Dog : Loud {
    override val name = "dog"
}

class Hush : Silent, Quiet {
    override val name = "hush"
    override fun twice() = "shh"
}

object Reg : Registry {
    override fun register(name: String, eager: Boolean) = name + " " + eager
}

fun show(cls: Class<*>) {
    val ms = cls.declaredMethods.map {
        var s = Modifier.toString(it.modifiers)
        if (it.isBridge) s += " bridge"
        if (it.isSynthetic) s += " synthetic"
        it.name + it.parameterTypes.joinToString(",", "(", ")") { p -> p.simpleName } + " " + s
    }
    println(cls.name + ": " + ms.sorted())
}

fun main() {
    val dog = Dog()
    println(dog.greeting + " " + dog.describe() + " " + dog.shout() + " " + dog.twice() + " " + dog.echo(3))
    dog.volume = 5
    println(dog.volume)
    val hush = Hush()
    println(hush.shout() + " " + hush.twice() + " " + hush.whisper())
    println(Leaf().describe("*") + " " + Leaf().twice())
    println(Reg.register("a") + " " + Reg.register("b", true))

    for (name in listOf("Named", "Loud", "Silent", "Quiet", "Registry")) {
        val impls = try {
            Class.forName("lib.$name\$DefaultImpls")
        } catch (e: ClassNotFoundException) {
            println("no lib.$name\$DefaultImpls")
            continue
        }
        show(impls)
    }
    show(Base::class.java)
    show(Leaf::class.java)
    show(Dog::class.java)
    show(Hush::class.java)

    // As code compiled against DefaultImpls calls them.
    val impls = Class.forName("lib.Loud\$DefaultImpls")
    println(impls.getMethod("twice", Loud::class.java).invoke(null, dog))
    println(impls.getMethod("getGreeting", Loud::class.java).invoke(null, dog))
    println(impls.getMethod("describe", Loud::class.java, String::class.java).invoke(null, dog, "#"))
    val reg = Class.forName("lib.Registry\$DefaultImpls")
    println(reg.getMethod("register\$default", Registry::class.java, String::class.java, Boolean::class.java, Int::class.java, Any::class.java).invoke(null, Reg, "c", true, 2, null))
}
