package q;
public interface Req<T> extends Cloneable {
    Req<T> clone();
    String name();
}
