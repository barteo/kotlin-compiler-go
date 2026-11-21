package p

import p.Outer.Nested.Entry // ERROR cannot access 'Outer.Nested'

fun otherFile() {
    makeA().bar() // ERROR cannot access 'bar': it is a member of 'A', which is private in file
    makeA().p // ERROR cannot access 'p': it is a member of 'A', which is private in file
    Entry // ERROR cannot access 'Entry'
}

fun outside(
    p: Holder.Prot, // ERROR cannot access 'Holder.Prot'
) {
    p.prot() // ERROR cannot access 'prot'
}
