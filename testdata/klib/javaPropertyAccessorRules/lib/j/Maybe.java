package j;
public class Maybe {
    private final boolean empty;
    public Maybe(boolean empty) { this.empty = empty; }
    public Single<Boolean> isEmpty() { return new Single<>(empty); }
}
