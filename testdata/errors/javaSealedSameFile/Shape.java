public abstract sealed class Shape {
    public static final class Circle extends Shape {}
    public static abstract sealed class Poly extends Shape {
        public static final class Tri extends Poly {}
        public static non-sealed class Quad extends Poly {}
    }
}
