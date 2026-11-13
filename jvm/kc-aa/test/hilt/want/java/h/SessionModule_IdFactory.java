package h;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

@ScopeMetadata("h.SessionScoped")
@QualifierMetadata
@DaggerGenerated
@Generated(
    value = "dagger.internal.codegen.ComponentProcessor",
    comments = "https://dagger.dev"
)
@SuppressWarnings({
    "unchecked",
    "rawtypes",
    "KotlinInternal",
    "KotlinInternalInJava",
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class SessionModule_IdFactory implements Factory<Long> {
  @Override
  public Long get() {
    return id();
  }

  public static SessionModule_IdFactory create() {
    return InstanceHolder.INSTANCE;
  }

  public static long id() {
    return SessionModule.INSTANCE.id();
  }

  private static final class InstanceHolder {
    static final SessionModule_IdFactory INSTANCE = new SessionModule_IdFactory();
  }
}
