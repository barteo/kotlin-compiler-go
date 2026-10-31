// Line breaks and annotations that kotlinpoet's generated code and Compose
// code use: modifiers and property types wrapped onto the next line, and
// annotated or wrapped type arguments of calls.

@Target(AnnotationTarget.TYPE)
annotation class Tag

class Scope(val n: Int)

class Holder<A, B>(val a: A, val b: B)

fun <A, B> make(a: A, b: B): Holder<A, B> = Holder(a, b)

public
    val wrapped:
    String = "wrapped"

public val wrappedType:
    Int = 42

public class
    WrappedClassName {
  public
      fun
      name(): String = "W"
}

object Meta {
  public
      fun hello(): String = "hello"

  internal
      val count: Int = 3
}

fun main() {
    val fs = mutableListOf<@Tag () -> String>()
    fs.add { "a" }
    val gs = mutableListOf<@Tag (Int) -> Int>()
    gs.add { it + 1 }
    val hs = mutableListOf<
        @Tag Scope.(x: Int, label: String) -> String
        >()
    hs.add { x, label -> label + (n + x) }
    val h = make<
        String,
        Int
        >("b", 2)
    val local:
        Long = 7L
    println(fs[0]() + gs[0](1) + hs[0](Scope(1), 2, "s"))
    println(h.a + h.b)
    println(wrapped + wrappedType + Meta.hello() + Meta.count + local + WrappedClassName().name())
}
