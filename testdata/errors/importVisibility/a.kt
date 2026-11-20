package a

// Another file's private declarations can't be imported, except
// typealiases and private and protected nested classes: their uses are
// reported.

private class Hidden
private typealias HiddenAlias = Int
private fun hidden() {}
private fun overloaded(x: Int) {}
fun overloaded() {}
private val hiddenVal = 1

class Visible {
    private class Nested
}

class WithPrivate {
    private class PrivateNested {
        object O
    }
    private object PrivateObject {
        fun f() {}
    }
    protected class Prot {
        class InProt
    }
    private companion object {
        fun g() {}
    }

    private fun ownCompanion(): Any = WithPrivate // its own private companion: visible
}

private class HiddenOuter {
    object Inner {
        fun h() {}
    }
}
