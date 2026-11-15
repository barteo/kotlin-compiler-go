import java.util.*;
import kotlin.coroutines.Continuation;

public class A<R> {
    public static void foo(Iterable<? super CharSequence> x) {}
    public static void bar(Iterator<? super CharSequence> x) {}
    public static String baz(int x) { return ""; }
    public <T> Object susp(T p, Continuation<? super T> c) { return p; }
    public Object raw(Continuation c) { return null; }
    public static Object notSuspend(Continuation<String> c) { return null; }
}
