// The synthetic properties of Java getters are named as K2's: getURL() is
// url, as decapitalized back, and only ASCII letters change case. A
// generic getter, and an annotation's element, make none, and two getters
// of one property are ambiguous.
fun use(n: Names) {
    n.url = n.url + "/"
    n.htmlFile += "1"
    n.URL // ERROR unresolved reference
    n.uRL // ERROR unresolved reference
    n.hTMLFile // ERROR unresolved reference
    n.КУ
    n.ку // ERROR unresolved reference
    n.generic // ERROR unresolved reference
    n.ter // ERROR unresolved reference
    n.vmParameters // ERROR overload resolution ambiguity
    n.vMParameters // ERROR unresolved reference
}

fun query(a: AnnoImpl) = a.query // ERROR unresolved reference
