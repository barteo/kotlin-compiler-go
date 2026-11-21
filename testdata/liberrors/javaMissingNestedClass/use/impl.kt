// deps: lib2
import a.d.Derived

// Without lib on the class path, Base.nested's parameter, a/b/C$Some in
// its descriptor, is a class of its own to K2, not Derived.nested's
// a/b/C.Some, so Derived doesn't implement it; Base.top's a/b/Top is
// Derived.top's.
class Impl : Derived() // ERROR does not implement abstract member 'nested'
