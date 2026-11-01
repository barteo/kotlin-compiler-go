import a.*

@All(i = 5)
class Annotated

fun main() {
    println(Two<String, Int>("two").s)
    println(Nesting())
    println(Nesting().a2.s)
    println(WithKClass(String::class).k == String::class)
    val all = All()
    println(all)
    println(all.k.java)
    println(all.ks.map { it.java })
    println(all.u)
    println(All(i = 5) == Annotated::class.java.getAnnotation(All::class.java))
    println(All(i = 5).hashCode() == Annotated::class.java.getAnnotation(All::class.java).hashCode())
    println(All() == All(i = 5))
    println(Outer())
    println(Outer.Inner())
    println(SinceKotlin("1.6.0"))
}
