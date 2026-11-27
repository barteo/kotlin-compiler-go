public abstract class FinalCharAt implements CharSequence {
    public final int length() { return 0; }
    public final char charAt(int index) { return 'K'; }
    public final CharSequence subSequence(int startIndex, int endIndex) { return this; }
    public abstract char get(int index);
}
