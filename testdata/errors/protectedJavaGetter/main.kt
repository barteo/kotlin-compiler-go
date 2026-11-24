// A synthetic property whose Java getter is protected is read only where
// the getter can be called: on this, not on another instance of a
// superclass.
package k

import j.Super

class Sub : Super() {
    fun f(s: Super, t: Sub) {
        s.name // ERROR cannot access 'getName'
        s.name = "x"
        t.name
        name
        this.name
    }
}
