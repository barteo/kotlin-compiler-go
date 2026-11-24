// kotlinc: -jvm-default=no-compatibility
// A generic interface with default methods and no DefaultImpls class,
// whose members an interface of the program overrides with its type
// argument.
interface Foo<T> {
    fun test(p: T) = "fail"
    val T.prop: String
        get() = "fail"
    fun keep(p: T) = "keep " + p
}
