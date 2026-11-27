package p;

public class Key<T> {
    private final String name;

    public Key(String name) { this.name = name; }

    public static <T> Key<T> create(String name) { return new Key<T>(name); }

    @Override public String toString() { return name; }
}
