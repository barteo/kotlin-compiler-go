// CLASSIFIER_REDECLARATION: a class the Java sources declare too.

class Dup { // ERROR redeclaration
    class Nested // ERROR redeclaration
    class OnlyKotlin
}

object OtherObject

fun use(o: Other) {}
