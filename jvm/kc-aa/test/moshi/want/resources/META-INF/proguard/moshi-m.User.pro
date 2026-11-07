-keepnames class m.User
-if class m.User
-keep class m.UserJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class m.User
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class m.User {
    public synthetic <init>(long,java.lang.String,java.lang.String,java.util.List,m.Role,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
