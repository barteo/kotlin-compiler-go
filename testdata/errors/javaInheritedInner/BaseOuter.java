public class BaseOuter<H> {
    public abstract class BaseInner<E, F> {
        public H foo1() { return null; }
        public E foo2() { return null; }
    }
    public class Own {
        public H get() { return null; }
    }
    public Own own() { return null; }
}
