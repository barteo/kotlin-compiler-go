-keepnames class m.Settings
-if class m.Settings
-keep class m.SettingsJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class m.Settings
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class m.Settings {
    public synthetic <init>(java.util.Map,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
