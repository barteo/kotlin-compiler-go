package p

public class A
public interface IA
public interface C<T>
public interface D<T>
public class F<T>
public fun foo(a: A) {}
public fun ia(a: IA) {}
public fun c(c: C<Int>) {}
public fun d(d: D<Int>) {}
public fun f(f: F<Int>) {}

public interface Base {
    public fun <T : Base> foo(t: Array<T>) {}
}
public interface BaseB : Base
