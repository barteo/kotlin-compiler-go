fun foo0(f: () -> String) = f
fun foo1(f: (Int) -> String) = f
fun foo2(f: (Int, String) -> String) = f

fun test() {
    foo0 { // ERROR expected no parameters, but 2 were declared
        x, // ERROR cannot infer a type for this parameter
        y -> "" // ERROR cannot infer a type for this parameter
    }
    foo1 { // ERROR expected 1 parameter of type Int, but 2 were declared
        x,
        y -> "" // ERROR cannot infer a type for this parameter
    }
    foo1 { s: String -> "" } // ERROR expected parameter of type Int
    foo2 { s: String -> "" } // ERROR expected 2 parameters of types Int, String, but 1 were declared
}
