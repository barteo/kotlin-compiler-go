import java.beans.ConstructorProperties

@SuppressWarnings("unchecked", "rawtypes")
fun f() = "f"

class P @ConstructorProperties("a", "b") constructor(val a: Int, val b: Int)

class Q @ConstructorProperties(value = ["c"]) constructor(val c: Int)

fun main() {
    println(f())
    println(P::class.java.constructors[0].getAnnotation(ConstructorProperties::class.java).value.joinToString())
    println(Q::class.java.constructors[0].getAnnotation(ConstructorProperties::class.java).value.joinToString())
}
