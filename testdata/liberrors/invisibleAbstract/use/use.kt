import base.*

class ImplDirectFromBase : Base() // ERROR inherits invisible abstract member 'internalFoo' of Base

object ImplObj : Base() // ERROR inherits invisible abstract member 'internalFoo' of Base

class ImplWithOverride : BaseWithOverride()

class ImplViaIntermediate : Intermediate() // ERROR inherits invisible abstract member 'internalFoo' of Base

abstract class StillAbstract : Base()

class FromConcrete : Concrete()

fun use() = ImplWithOverride().foo() + FromConcrete()

// Not internalFoo, which it can't override, but internalProp.
class TriesToOverride : Base() { // ERROR inherits invisible abstract member 'internalProp' of Base
    override fun internalFoo(): String = "" // ERROR 'internalFoo' overrides nothing
}
