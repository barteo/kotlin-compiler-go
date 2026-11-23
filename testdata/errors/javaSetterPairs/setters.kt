// A Java property's setter, as K2 pairs it with the getter: not a
// generic method, nor one whose parameter's nullability differs from
// the getter's, unless it overrides one that is the setter (KT-56506).
fun use(j: JavaClass, o: JavaOverride, s: String) {
    j.generic++ // ERROR val cannot be reassigned
    j.plain++
    j.name = s // ERROR val cannot be reassigned
    o.foo = s
}
