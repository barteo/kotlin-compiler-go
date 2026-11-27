package a

object O {
    class A
    object B

    fun bar() {}
}

object S {
    fun A(c: Int) = A()

    class A()
}
