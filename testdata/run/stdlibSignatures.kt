// Generic signatures of classes, which Java reflection (and javac) read:
// type parameters and their bounds, generic superclasses and interfaces,
// with the wildcards of declaration-site variance in nested arguments.
import java.io.Serializable

open class G<T>
open class B<T : Comparable<T>, U : CharSequence, V>
interface TypeSerializer<T> : Comparable<T>, Serializable {
    abstract class Deserializer<T> : TypeSerializer<T>
}
class C1 : B<String, String, List<String>>()
class C2<X : Any> : B<Int, StringBuilder, Map<X, List<Int>>>()
class C3 : G<MutableList<in Number>>()
class C4 : G<Array<List<String>>>()
class C5 : G<IntArray>(), Comparable<Unit> {
    override fun compareTo(other: Unit) = 0
}
class C6 : G<List<*>>()
class C7 : G<(String) -> Int>()
class C8 : G<suspend (String) -> Int>()
class C9 : G<Nothing>()
class C10 : G<List<List<Nothing>>>()
class C11<T> where T : CharSequence, T : Comparable<T>
class C12<T : List<CharSequence>, U : T>
class C13 : (Int) -> String {
    override fun invoke(p: Int) = ""
}
abstract class C14<E> : MutableList<E>
abstract class C15 : List<String>
class Outer<T> {
    inner class Inner<U> : G<Pair<T, U>>()
}
enum class E { A }
sealed class Res<out T> {
    class Ok<T>(val v: T) : Res<T>()
    object Err : Res<Nothing>()
}
class Plain

fun describe(c: Class<*>) {
    val params = c.typeParameters.joinToString(", ") { p -> p.name + " : " + p.bounds.joinToString(" & ") }
    println(c.simpleName + "<" + params + ">")
    println("  extends " + c.genericSuperclass)
    for (i in c.genericInterfaces) {
        println("  implements " + i)
    }
}

fun main() {
    for (c in listOf(
        G::class.java, B::class.java, TypeSerializer::class.java, TypeSerializer.Deserializer::class.java,
        C1::class.java, C2::class.java, C3::class.java, C4::class.java, C5::class.java, C6::class.java,
        C7::class.java, C8::class.java, C9::class.java, C10::class.java, C11::class.java, C12::class.java,
        C13::class.java, C14::class.java, C15::class.java, Outer<Int>().Inner<Int>().javaClass, E::class.java,
        Res::class.java, Res.Ok::class.java, Res.Err::class.java, Plain::class.java,
    )) {
        describe(c)
    }
    val anon = object : Comparator<String> {
        override fun compare(a: String, b: String) = 0
    }
    describe(anon.javaClass)
}
