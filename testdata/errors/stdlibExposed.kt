// A delegated property's type is checked, not its delegate's.
private class Delegate {
    operator fun getValue(thisRef: Any?, p: Any?) = 1
}
val delegated: Int by Delegate()
val delegatedHidden by Hidden() // ERROR 'public' property exposes its 'internal' type 'Hidden'
internal class Hidden {
    operator fun getValue(thisRef: Any?, p: Any?) = this
}
