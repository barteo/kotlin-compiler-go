// x[i]++ on an x of a builder's type that isn't inferred yet: kotlinc
// resolves the get and the set of an increment without fixing the
// variable, as it does for other receivers (KT-71778), so neither is
// found, and the builder's type stays open: Other fits it afterwards.
// x[i] += v does fix it, for its get.
class Owner<T> {
    fun constrain(value: T) {}
    val reference: T = null!!
}

fun <OT> pcla(lambda: (Owner<OT>) -> Unit): OT = null!!

interface Base

object Index

class Scope : Base {
    operator fun inc(): Scope = this
    operator fun get(index: Index): Scope = this
    operator fun set(index: Index, value: Scope) {}
    operator fun plus(other: Scope): Scope = this
}

object Other : Base

fun postfix(): Base = pcla { owner ->
    owner.constrain(Scope())
    owner.reference[Index]++ // ERROR unresolved reference: get
    owner.constrain(Other)
}

fun prefix(): Base = pcla { owner ->
    owner.constrain(Scope())
    ++owner.reference[Index] // ERROR unresolved reference: get
    owner.constrain(Other)
}

fun safe(): Base = pcla { owner ->
    owner.constrain(Scope())
    owner?.reference[Index]-- // ERROR unresolved reference: get
    owner.constrain(Other)
}

fun compound(): Scope = pcla { owner ->
    owner.constrain(Scope())
    owner.reference[Index] += Scope()
}

fun compoundFixes() {
    pcla { owner ->
        owner.constrain(Scope())
        owner.reference[Index] += Scope()
        owner.constrain(Other) // ERROR type mismatch
    }
}
