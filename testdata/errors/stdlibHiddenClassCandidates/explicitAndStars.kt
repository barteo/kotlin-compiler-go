import p1.*
import p2.A
import p3.*

fun test(a: A) { // ERROR deprecated
    a.v1 // ERROR unresolved reference: v1
    a.v2 // ERROR unresolved reference: v2
    a.v3 // ERROR unresolved reference: v3
    a.toString()
    a.hashCode()
}
