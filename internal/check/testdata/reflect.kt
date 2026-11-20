// The warnings of uses of kotlin.reflect that need kotlin-reflect, which
// isn't on the class path, below, as TestWarningsReflection reads them:
// each line ends with a comment naming the warnings it reports, if any.
// They follow kotlinc 2.4.21's AbstractFirReflectionApiCallChecker and
// the test noReflectionInClassPath.kt of its diagnostics tests.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_EXPRESSION")

import kotlin.reflect.*

class Foo(val prop: Any) {
    fun func() {}
}

fun refs() {
    Foo::prop.getter // NO_REFLECTION_IN_CLASS_PATH
    Foo::class.members // NO_REFLECTION_IN_CLASS_PATH
    Foo::class.simpleName
    Foo::prop.getter(Foo(42)) // NO_REFLECTION_IN_CLASS_PATH
    (Foo::func).invoke(Foo(""))
    Foo::func.name
}

fun <T : Any> kclass(k: KClass<*>, kt: KClass<T>) {
    k.simpleName
    k.qualifiedName
    k.isInstance(42)
    k.constructors // NO_REFLECTION_IN_CLASS_PATH
    k.isData // NO_REFLECTION_IN_CLASS_PATH
    k.annotations // NO_REFLECTION_IN_CLASS_PATH
    k == kt
    k.hashCode()
    k.toString()
}

fun props(p: KProperty0<Int>, m: KMutableProperty0<String>, q: KProperty1<String, Int>, f: KFunction<String>) {
    p.get()
    m.set("")
    q.get("")
    f.name
    f.call() // NO_REFLECTION_IN_CLASS_PATH
    q.returnType // NO_REFLECTION_IN_CLASS_PATH
}

fun ktype(t: KType) {
    t.classifier
    t.arguments
    t.annotations
    KTypeProjection.covariant(t)
}

fun <T : KClass<*>> bounded(t: T, x: CharSequence) {
    t.members // NO_REFLECTION_IN_CLASS_PATH
    if (x is KClass<*>) {
        x.members // NO_REFLECTION_IN_CLASS_PATH
    }
}
