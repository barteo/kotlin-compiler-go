class A2(x: Byte) {
    constructor(x1: Int): this(x1, 1) // ERROR there's a cycle in the delegation calls chain
    constructor(x1: Int, x2: Int): this(x1) // ERROR there's a cycle in the delegation calls chain

    // into a cycle declared before or after
    constructor(x1: Double): this(1) // ERROR there's a cycle in the delegation calls chain
    constructor(x1: String): this(1L) // ERROR there's a cycle in the delegation calls chain
    constructor(x1: Long): this(x1, 1L) // ERROR there's a cycle in the delegation calls chain
    constructor(x1: Long, x2: Long): this(x1) // ERROR there's a cycle in the delegation calls chain

    // no cycle, just a chain to the primary constructor
    constructor(x1: Double, x2: Double): this(x1, x2, 1.0)
    constructor(x1: Double, x2: Double, x3: Double): this(1.toByte())
}

class K(x: Int) {
    constructor() : this() {} // ERROR there's a cycle in the delegation calls chain
    constructor(s: String) : this() // ERROR there's a cycle in the delegation calls chain
}

// constructor(x: T2) doesn't take { x }: T2 is fixed in this(...).
class L<T1, T2> {
    constructor(block: (T1) -> T2)
    constructor(x: T2): this({ x })
}
