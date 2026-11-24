import java.util.AbstractList;

@kotlin.jvm.PurelyImplements("kotlin.collections.MutableList")
public class A<T> extends AbstractList<T> {
    public T get(int index) { return null; }
    public int size() { return 0; }
    public void own(T t) {}
}
