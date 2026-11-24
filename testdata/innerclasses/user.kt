// The nested classes of outer.kt and of the JDK, named in annotations,
// descriptors, generic signatures and code (TestInnerClasses).
package q

annotation class Tagged(val kind: Outer.Kind, val type: kotlin.reflect.KClass<*>)

@Outer.Mark
@Outer.Experimental
class User(val nest: Outer.Nest) {
    @Tagged(Outer.Kind.B, Outer.Nest.Deeper::class)
    fun param(n: Outer.Nest): String = n.hi()

    fun entry(e: Map.Entry<String, Int>): Int = e.value

    fun make(): Any = Outer.Nest()

    fun simple(e: java.util.AbstractMap.SimpleEntry<String, Int>): String = e.key

    fun parts(): List<Generic<String>.Part> = emptyList()

    fun inner(o: Outer): Outer.Inner = o.Inner()

    fun name(): String = Outer.NAME

    fun callback(): Outer.Callback = Callback()
}

class Callback : Outer.Callback {
    override fun call() {}
}

fun kinds(): Array<Outer.Kind> = Outer.Kind.entries.toTypedArray()
