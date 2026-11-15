public record R<T>(T x, String name) implements I<T> {
    public int other() { return 1; }
}
