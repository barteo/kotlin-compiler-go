// As in K2, a name that is a classifier in scope names it, not a
// package of that name: in package q, p.b is class q.p's b, which
// doesn't exist, not class p.b.
package q

class p

fun f() {
    val x = p.b() // ERROR unresolved reference: b
}
