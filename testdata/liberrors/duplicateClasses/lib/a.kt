package p

// Classes of the names lib2 declares too: the class path's first ones.
public class A<T>
public interface IA<T>
public interface C
public interface D<T>
public class B : IA<Int>, C, D<Int>
public class F
public class M1 {
    public val a: A<Int> = A<Int>()
    public val b: B = B()
    public val f: F = F()
}

public interface Base {
    public fun <T> foo(t: Array<T>) {}
}
public interface BaseA : Base
