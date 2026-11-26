// The root package's classes, which another package's files see only as
// the qualifiers of their members and nested classes.
open class Klass {
    fun klassFunction() = "klassFunction"

    open class Nested {
        fun nestedFunction() = "nestedFunction"
    }

    object NestedObject {
        fun nestedObjectFunction() = "nestedObjectFunction"
    }

    companion object {
        fun companionFunction() = "companionFunction"
    }
}

object Objekt {
    const val NAME = "Objekt"

    fun objektFunction() = "objektFunction"

    class ObjektNested {
        override fun toString() = "ObjektNested"
    }
}

typealias KlassAlias = Klass
typealias ObjektAlias = Objekt
