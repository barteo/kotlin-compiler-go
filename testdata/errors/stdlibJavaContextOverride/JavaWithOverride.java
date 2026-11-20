public class JavaWithOverride implements KotlinContextInterface {
    @Override
    public String foo(String a, int b) {
        return a + b;
    }

    @Override
    public double getBar(String a) {
        return 3.14;
    }
}
