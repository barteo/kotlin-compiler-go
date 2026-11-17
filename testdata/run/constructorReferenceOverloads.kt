class Services

abstract class Handler(val services: Services)

class Plain(services: Services) : Handler(services)

class Named(services: Services, val name: String) : Handler(services)

class Both(services: Services) : Handler(services) {
    var name = ""

    constructor(services: Services, name: String) : this(services) {
        this.name = name
    }
}

// A constructor reference picks the overload whose function type one of
// the class's constructors has.
class Builder<Kind> {
    fun use(vararg constructors: (Services) -> Handler) = "one parameter: " + constructors.size

    fun use(vararg constructors: (Services, Kind) -> Handler) = "two parameters: " + constructors.size
}

fun build(make: (Services) -> Handler) = "one"

fun build(make: (Services, String) -> Handler) = "two"

fun main() {
    println(Builder<String>().use(::Plain))
    println(Builder<String>().use(::Named))
    println(Builder<String>().use(::Plain, ::Both))
    println(build(::Plain) + " " + build(::Named))
}
