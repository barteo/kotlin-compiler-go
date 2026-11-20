public interface Function1<A extends B, B extends A> {
    B handle(A a);
}
