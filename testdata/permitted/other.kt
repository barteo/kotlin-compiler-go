package p

// Subclasses in another file of the module.
class Other : Expr()

data object Dobj : Shape

sealed class Mid : Marker {
    class Leaf : Mid()
}
