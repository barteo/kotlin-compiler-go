-keepnames class m.Page
-if class m.Page
-keep class m.PageJsonAdapter {
    public <init>(com.squareup.moshi.Moshi,java.lang.reflect.Type[]);
}
