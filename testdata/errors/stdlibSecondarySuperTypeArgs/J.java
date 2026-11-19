import java.util.List;

public class J<T> {
    public <U extends T> J(T t, U u) {}
    public J(List<T> l) {}
}
