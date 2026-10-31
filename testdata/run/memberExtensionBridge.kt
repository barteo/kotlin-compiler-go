// An override of a member extension whose receiver is a type parameter
// gets a bridge, as with kotlinc: the interface calls bar(Object).

class Foo {
    fun bar() = "member"
}

interface Extension<T> {
    fun T.bar(): String

    fun call(t: T) = t.bar()
}

class FooExtension : Extension<Foo> {
    override fun Foo.bar() = "extension"
}

fun main() {
    println(FooExtension().call(Foo()))
    println(Foo().bar())
}
