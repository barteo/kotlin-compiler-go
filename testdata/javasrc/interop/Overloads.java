package interop;

import kotlin.Unit;
import kotlin.jvm.functions.Function1;

public class Overloads {
    interface Listener {
        void invoke(Object o);
    }

    public String foo(Fn<String, Object> f, Object o) { return "foo(Fn<String>) " + f.apply("s"); }
    public String foo(Fn<Object, Object> f, String s) { return "foo(Fn<Object>) " + f.apply(1); }

    public String bar(Fn<String, Object> f) { return "bar(Fn) " + f.apply("s"); }
    public String bar(Fn2<String, Object> f) { return "bar(Fn2) " + f.apply("s"); }

    public String test(Listener l) { l.invoke("x"); return "test(Listener)"; }
    public String test(Function1<Integer, Unit> f) { return "test(Function1)"; }
}
