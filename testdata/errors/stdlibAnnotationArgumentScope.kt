// Annotation arguments are resolved where the annotated declaration is:
// those of a member or its parameters see the class's instance members,
// and those of an inner class and its supertypes, or of a local class's
// members, the outer class's. A nested class's don't, nor do those of a
// constructor and its parameters, which see the constructor's parameters
// (see ctorHeaderAnnotations.kt).
@Target(AnnotationTarget.CLASS, AnnotationTarget.FUNCTION, AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.TYPE, AnnotationTarget.PROPERTY)
annotation class Anno(val value: Int)

const val CONST = 1
const val OWN = 1

class MyClass {
    val CONST = ""

    @Anno(CONST) // ERROR type mismatch
    inner class InnerClass(
        @Anno(CONST) val a: String, // ERROR type mismatch
    ) : @Anno(CONST) Any() { // ERROR type mismatch
        val OWN = ""
        @Anno(OWN) fun foo() {} // ERROR type mismatch
        @Anno(CONST) val p = 1 // ERROR type mismatch

        inner class Deeper {
            @Anno(CONST) fun bar() {} // ERROR type mismatch
        }
    }

    @Anno(CONST)
    class NestedClass(@Anno(CONST) val a: String) : @Anno(CONST) Any() {
        @Anno(CONST) fun foo() {}

        inner class InNested {
            @Anno(CONST) fun q() {}
        }
    }

    fun local() {
        class L {
            @Anno(CONST) fun r() {} // ERROR type mismatch
        }
    }
}

class Gen<T> {
    val CONST = 2
    inner class I {
        @Anno(CONST) fun s() {} // ERROR must be a compile-time constant
    }
}

class X {
    val CONST = ""
    @Anno(CONST) fun f() {} // ERROR type mismatch
    fun g(@Anno(CONST) x: Int) {} // ERROR type mismatch
    companion object {
        const val OWN = 2
    }
    @Anno(OWN) fun h() {}
}

@Anno(CONST)
class Y(@Anno(CONST) val CONST: String) { // ERROR type mismatch
    @Anno(CONST) fun k() {} // ERROR type mismatch
}

object O {
    val CONST = ""
    @Anno(CONST) fun m() {} // ERROR type mismatch
}
