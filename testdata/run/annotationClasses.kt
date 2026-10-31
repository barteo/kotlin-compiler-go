annotation class Marker

annotation class Tag(val name: String, val level: Int = 1)

enum class Level { LOW, HIGH }

annotation class Rated(val level: Level, val inner: Marker, val scores: IntArray, val names: Array<String>)

annotation class Many(vararg val values: String)

class Outer {
    annotation class Nested(val v: Long)
}

@Marker
@Tag("service", level = 2)
class Service @Marker constructor(@Tag("p") val port: Int) {
    @Rated(Level.HIGH, Marker(), [1, 2], ["a"])
    @Many("x", "y")
    fun run(): Int = port + 1

    @Outer.Nested(1L)
    val name: String = "svc"
}

fun describe(c: Class<*>) {
    // Not Modifier.toString: kc writes no InnerClasses attribute, which
    // makes a nested class static.
    val m = c.modifiers
    println(c.name + ": public " + java.lang.reflect.Modifier.isPublic(m) + ", abstract " + java.lang.reflect.Modifier.isAbstract(m) +
        ", interface " + c.isInterface() + ", annotation " + c.isAnnotation())
    for (i in c.interfaces) {
        println("  implements " + i.name)
    }
    val methods = c.declaredMethods
    val names = arrayOfNulls<String>(methods.size)
    var i = 0
    while (i < methods.size) {
        names[i] = methods[i].name + "(): " + methods[i].returnType.name
        i++
    }
    java.util.Arrays.sort(names)
    for (n in names) {
        println("  " + n)
    }
}

fun main() {
    val s = Service(8080)
    println(s.run())
    println(s.name)
    describe(Marker::class.java)
    describe(Tag::class.java)
    describe(Rated::class.java)
    describe(Many::class.java)
    describe(Outer.Nested::class.java)
}
