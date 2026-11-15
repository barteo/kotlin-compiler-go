import JavaClass.*
import kotlin.reflect.*

// As in K2, a Java class's static field is a property without a
// receiver, KProperty0, and an enum's entries one too; an enum's entry
// isn't a property to refer to.
fun fields() {
    val a = JavaClass::publicFinal
    val b: KProperty0<String> = JavaClass::publicFinal
    val c: KMutableProperty0<Any?> = ::publicMutable
    val d: KProperty1<JavaClass, String?> = JavaClass::nonStaticField
    val e = ::privateMutable // ERROR privateMutable
    val f: KMutableProperty0<String?> = JavaClass()::nonStaticField
    val g: String = JavaClass::publicFinal.get()
}

enum class Some { A }

enum class Own {
    ;
    val entries: Int = 0
}

enum class SomeClass {
    FIRST;
    class entries
}

fun enums() {
    val x = Some::entries
    val k: KProperty0<kotlin.enums.EnumEntries<Some>> = Some::entries
    val own: KProperty1<Own, Int> = Own::entries
    val y = SomeClass::entries // ERROR overload resolution ambiguity
}
