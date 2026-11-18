// An override can't have reified type parameters
// (REIFIED_TYPE_PARAMETER_IN_OVERRIDE): a call through the overridden
// function couldn't pass the type.
interface IBase {
    fun <T> qux(x: T)
}

class CDerived : IBase {
    override inline fun <reified T> qux(x: T) {} // ERROR override by a function with reified type parameter
    inline fun <reified T> own(x: T) {}
}
