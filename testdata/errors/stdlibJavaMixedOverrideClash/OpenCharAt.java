public abstract class OpenCharAt implements CharSequence {
    public final int length() { return 0; }
    public char charAt(int index) { return 'K'; }
    public final CharSequence subSequence(int startIndex, int endIndex) { return this; }
    public abstract char get(int index);
}
