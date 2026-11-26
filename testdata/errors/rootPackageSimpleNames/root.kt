// The root package's classes and typealiases, which another package's
// files find as qualifiers only (see testdata/run/rootPackageQualifiers).
class Klass {
    class Nested

    companion object {
        fun companionFunction() = 1
    }
}

object Objekt

typealias KlassAlias = Klass
