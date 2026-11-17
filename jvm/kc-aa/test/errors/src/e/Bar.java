package e;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.Callable;

@Bad
public class Bar {
    Foo foo;

    static String read() throws IOException {
        return "";
    }

    static <T> T call(Callable<T> c) throws Exception {
        return c.call();
    }

    static <T> List<? extends T> wrap(T t) {
        return List.of(t);
    }

    static <T> List<T> copy(List<? extends T> list) {
        return List.copyOf(list);
    }

    int length() throws Exception {
        return call(Bar::read).length() + copy(wrap("")).size();
    }
}
