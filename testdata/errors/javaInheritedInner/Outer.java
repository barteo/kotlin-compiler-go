public class Outer<H> extends BaseOuter<H> {
    public BaseInner<Double, String> bar() { return null; }
    public class Inner extends BaseOuter<H>.BaseInner<Double, String> {}
}
