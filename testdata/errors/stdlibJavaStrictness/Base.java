public sealed abstract class Base permits A, B {
    public static Base provide() { return new A(); }
}
