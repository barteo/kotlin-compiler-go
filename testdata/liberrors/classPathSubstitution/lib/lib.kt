package p

class A
class B {
    val a: A = A()
}

interface Base
abstract class Foo : Base

interface ToSubstitute
interface Intermediate : ToSubstitute
