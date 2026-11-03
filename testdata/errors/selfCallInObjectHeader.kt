open class Base(val prop: Int, val text: String)

open class Foo1(prop: Int, text: String) : Base(prop, text) {
    companion object : Foo1(prop, text) // ERROR self call in nested object constructor is prohibited
}

open class Foo2(prop: Int, text: String) : Base(prop, text) {
    companion object : Foo2(this.prop, Companion.text) // ERROR self call in nested object constructor is prohibited
}

open class Foo3(prop: Int, text: String) : Base(prop, text) {
    object Inner : Foo3(Inner.prop, "x") // ERROR self call in nested object constructor is prohibited
}

// In a lambda, which runs later, it may.
open class Lazy(val f: () -> Int) {
    companion object : Lazy({ Companion.hashCode() })
}
