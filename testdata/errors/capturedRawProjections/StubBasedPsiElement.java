public interface StubBasedPsiElement<T extends StubElement> {
    T foo1();

    StubElement foo2();

    static StubElement create() { return null; }
}
