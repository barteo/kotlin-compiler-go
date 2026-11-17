package k;

public class JavaService {
    private final Clock clock;

    public JavaService(Clock clock) {
        this.clock = clock;
    }

    public Clock getClock() {
        return clock;
    }
}
