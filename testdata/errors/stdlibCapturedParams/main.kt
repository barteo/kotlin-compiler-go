// What kotlinc 2.4.21 reports of arguments passed for a parameter whose
// type has an invariant argument that a star or an in projection of the
// receiver replaces, Kotlin's captured type, which takes no other type;
// and of null passed for a Java T, which makes T Nothing? if nothing else
// decides it.

class KC<E> {
    fun set(e: E) {}
    fun setW(w: Wrapper<E>) {}
    fun setL(w: List<E>) {}
}

fun star(k: KC<*>, w: Wrapper<String>, l: List<String>, c: Container<*>) {
    k.setW(w) // ERROR type mismatch
    k.setL(l) // ERROR type mismatch
    c.setWrapper(w) // ERROR type mismatch
    c.wrapper = w // ERROR type mismatch
}

fun inProjected(k: KC<in String>, w: Wrapper<String>, a: Wrapper<Any>, c: Container<in String>) {
    k.set("")
    k.setW(w) // ERROR type mismatch
    k.setW(a) // ERROR type mismatch
    c.setWrapper(a) // ERROR type mismatch
    c.wrapper = a // ERROR type mismatch
}

fun outProjected(k: KC<out Any>, w: Wrapper<String>) {
    k.setW(w) // ERROR type mismatch
}

fun invariant(k: KC<String>, w: Wrapper<String>, c: Container<String>) {
    k.setW(w)
    c.setWrapper(w)
    c.wrapper = w
}

fun takeString(s: String) {}

fun javaNull() {
    takeString(JavaBox(null).a) // ERROR type mismatch
    takeString(JavaBox.same(JavaBox(null)).a) // ERROR type mismatch
    val b = JavaBox(null)
    val n: Int = b.a // ERROR type mismatch
    val s: JavaBox<String> = JavaBox(null)
    takeString(JavaBox("").a)
}

// As K2 (KT-67825): null for a Java T! makes T Nothing? against a bound
// that explicit type arguments give, not one written or substituted from
// the receiver's type.
class NullBoxes<R> {
    fun take(a: JavaBox<out R>) {}
}

fun <S> takeBoxOf(a: JavaBox<out S>) {}

fun takeStringBox(a: JavaBox<out String>) {}

fun explicitNull(k: NullBoxes<String>) {
    k.take(JavaBox(null))
    takeBoxOf<String>(JavaBox(null)) // ERROR type mismatch
    takeStringBox(JavaBox(null))
}
