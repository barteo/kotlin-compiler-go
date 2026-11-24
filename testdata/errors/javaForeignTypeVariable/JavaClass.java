public class JavaClass<T> {
    public <X> Inner<X> createInner() {
        return new Inner<X>();
    }

    public interface Inner<X> {
        public T doSomething(T t, X x, Runnable runnable);
    }
}
