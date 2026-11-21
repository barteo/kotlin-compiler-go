import java.util.List;

public abstract class A extends AImpl implements List<String> {
    void bar(List<? super Object> x) {}
}
