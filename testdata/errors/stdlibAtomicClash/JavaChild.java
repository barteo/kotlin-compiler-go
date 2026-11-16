import java.util.concurrent.atomic.AtomicInteger;

public class JavaChild extends Base {
    @Override
    public void foo(AtomicInteger a) { }

    @Override
    public AtomicInteger getA() {
        return new AtomicInteger(1);
    }
}
