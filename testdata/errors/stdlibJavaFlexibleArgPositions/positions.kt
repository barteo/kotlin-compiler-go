// A nullable argument passed for a Java parameter of a variable's flexible
// type, String? <: T!, holds with an upper bound String that an argument's
// parameter type gives, T! <: String: as K2, kl(J.listOf(s)) is an argument
// of a List<String!>. Not where a declaration, a return or an assignment
// expects the type, nor through explicit type arguments that aren't
// flexible in the parameter's type, nor an indexing operator's arguments.

import java.util.Collections

fun kl(l: List<String>) {}
fun <S> kg(b: JavaBox<S>) {}
class P(val l: List<String>)

fun accepted(s: String?, lst: java.util.ArrayList<List<String>>, m: java.util.HashMap<String, JavaBox<String>>) {
    kl(J.listOf(s))
    kl(l = J.listOf(s))
    P(J.listOf(s))
    kl(Collections.singletonList(s))
    J.jl(J.listOf(s))
    lst.add(J.listOf(s))
    lst.add(Collections.singletonList(s))
    lst.add(java.util.ArrayList(listOf(s)))
    m.put("a", J.box(s))
    m.put("a", JavaBox(s))
    m.put("a", JavaBox(null))
    J.jb(J.box(s))
    J.jb(JavaBox(null))
    kl(Collections.singletonList(null))
    lst.add(Collections.singletonList(null))
    val x: List<String> = Collections.singletonList(null)
    N.p<String>(JavaBox(s))
    N.pl<String>(Collections.singletonList(s))
    kg(JavaBox(s))
}

fun rejected(s: String?, c: Boolean, ml: java.util.HashMap<String, List<String>>) {
    val v: List<String> = J.listOf(s) // ERROR type mismatch
    kl(if (c) J.listOf(s) else listOf()) // ERROR type mismatch
    ml["a"] = Collections.singletonList(s) // ERROR type mismatch
    kg<String>(JavaBox(s)) // ERROR type mismatch
    kg<String>(JavaBox(null)) // ERROR type mismatch
    N.nn<String>(JavaBox(s)) // ERROR type mismatch
    N.nn<String>(JavaBox(null)) // ERROR type mismatch
    N.nl<String>(Collections.singletonList(s)) // ERROR type mismatch
    N.nl<String>(Collections.singletonList(null)) // ERROR type mismatch
    N.nn(JavaBox(s)) // ERROR not within its bounds
    listOf<List<String>>(J.listOf(s)) // ERROR type mismatch
}

fun returned(s: String?): List<String> = J.listOf(s) // ERROR type mismatch

// At the top level, T! <: String, null passed for T! yields to a
// declaration's type, not to an argument's parameter type, and a nullable
// argument to neither.
fun eat(s: String) {}

fun topLevel(s: String?) {
    val x: String = J.transform(null)
    J.jl(J.transform(null))
    eat(J.transform(null)) // ERROR type mismatch
    eat(J.transform(s)) // ERROR type mismatch
    val y: String = J.transform(s) // ERROR type mismatch
}
