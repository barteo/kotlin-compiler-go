// The warnings kotlinc 2.4.21 reports of the misplaced type arguments of
// the callable references' left sides below, as TestWarningsLHSArgs
// reads them: each line ends with a comment naming the warnings it
// reports, if any.
class Outer<A> {
    inner class Middle<B> {
        inner class Inner<C>
    }
    class Nested<D> {
        inner class In<E>
    }
}

fun Outer<Int>.Middle<String>.Inner<Char>.foo() {}

val r1 = Outer<Int>.Middle<String>.Inner<Char>::foo
val r2 = Outer<Int>.Middle.Inner<Char, String>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val r3 = Outer.Middle.Inner<Char, String, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val r4 = Outer<Char, String, Int>.Middle.Inner::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val r5 = Outer.Middle<Int>.Inner<Char, String>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val r6 = Outer.Nested.In<Int, String>::hashCode // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val r7 = Outer.Nested<Int>.In<String>::hashCode

class WithParenthesizes<T> {
    inner class A<K> {
        fun foo() {}
    }
}

val p1 = (WithParenthesizes<String, Int>.A)::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val p2 = (WithParenthesizes<String, Int>).A::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val p3 = (WithParenthesizes.A<String, Int>)::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val p4 = (WithParenthesizes).A<String, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val p5 = (WithParenthesizes<String>).A<Int>::foo
val p6 = WithParenthesizes.A<String, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val p7 = WithParenthesizes<String>.A<Int>::foo

open class X<A> {
    inner class Y<B> {
        fun foo() {}
    }

    private val ref = Y<Int, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING

    class Z {
        val ref = Y<Int, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
    }
}

class T : X<String>() {
    val ref = Y<Int, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
}

fun <C> local(): Any {
    class X<A> {
        inner class Y<B> {
            fun foo() {}
        }

        val ref = Y<Int, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING
    }
    return X<String>()
}

// Type arguments of a class an inner class's nested class is in are an
// error, but taken for the nested class's with the warning.
class Top<T> {
    class Mid<B, A> {
        inner class Low<C>
    }
}

fun Top.Mid<String, Int>.Low<Char>.bar() {}

val n1 = Top<Int>.Mid<String>.Low<Char>::bar // error WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val n2 = Top<Char, String, Int>.Mid.Low::bar // error WRONG_NUMBER_OF_TYPE_ARGUMENTS_WARNING
val n3 = Top<Any>.Mid<String, Int>.Low<Char>::bar // error

// A local class takes the type parameters of the functions around it as
// in scope, and wants arguments only for its own.
fun <T> captures() {
    class LOuter<A> {
        inner class LInner<B> {
            fun foo() {}
        }
    }
    val c1 = LOuter<Int>.LInner<String>::foo
    val c2 = LOuter<String, Int>.LInner::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING
    val c3 = LOuter.LInner<String, Int>::foo // WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING
}

class InClass<U> {
    fun <T> foo() {
        class L

        val x = L::toString // error
        val y = L<Int>::toString // WRONG_NUMBER_OF_TYPE_ARGUMENTS_IN_LOCAL_CLASS_IN_LHS_WARNING
    }
}
