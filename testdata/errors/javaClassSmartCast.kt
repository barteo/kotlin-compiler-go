// x is java.lang.CharSequence keeps the Java class, whose length() is a
// method, as kotlinc does; kotlin.CharSequence's length is a property.
fun f(x: Any) {
    if (x is java.lang.CharSequence) {
        if ("a" == x || "b" == x) x.length else x.length()
        x.length()
    }
    if (x is CharSequence) {
        x.length() // ERROR expression 'length' of type Int cannot be invoked as a function
    }
}
