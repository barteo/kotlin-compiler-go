package j;
public class Options {
    private final Double pitch;
    public Options(Double pitch) { this.pitch = pitch; }
    public Double getPitch() { return pitch; }
    public static String take(Double d) { return "took " + d; }
}
