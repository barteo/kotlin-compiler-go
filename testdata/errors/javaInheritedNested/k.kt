open class KotlinClass : KotlinInterface.KotlinInner2() {
    inner class KotlinInner
}

interface KotlinInterface {
    open class KotlinInner2 : JavaClass2() {
        class KotlinInner3
    }
}

fun main() {
    val a: KotlinClass.KotlinInner = Use().kotlinInner
    val b: JavaClass2.JavaInner = Use().javaInner
    val c: KotlinInterface.KotlinInner2.KotlinInner3 = Use().kotlinInner3
    val d: String = Use().javaInner // ERROR type mismatch
}
