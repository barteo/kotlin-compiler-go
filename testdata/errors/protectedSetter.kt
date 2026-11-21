open class Base {
    open var y: Int = 1
        protected set
}

class Derived : Base() {
    fun f(x: Base, d: Derived, n: Base?) {
        x.y = 2 // ERROR cannot assign to 'y': the setter is protected in 'Base'
        x.y++ // ERROR cannot assign to 'y': the setter is protected in 'Base'
        x.y += 1 // ERROR cannot assign to 'y': the setter is protected in 'Base'
        d.y = 2
        y = 2
        this.y = 2
        if (x is Derived) {
            x.y = 3 // ERROR cannot assign to 'y': the setter is protected in 'Base'
        }
        if (n is Derived) {
            n.y = 3
        }
    }
}
