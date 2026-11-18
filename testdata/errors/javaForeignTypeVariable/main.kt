// A static nested Java interface whose method names its outer class's T,
// which javac rejects, has T unsubstituted, as with kotlinc.
fun foo(javaClass: JavaClass<Int>): Int {
    val inner = javaClass.createInner<String>()
    return inner.doSomething(1, "") { } // ERROR type mismatch
}
