-keepnames class m.Nested
-if class m.Nested
-keep class m.NestedJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
