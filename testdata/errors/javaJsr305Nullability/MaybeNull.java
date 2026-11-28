// A nickname of @Nonnull(when = MAYBE), as Spring's @Nullable is: kotlinc
// only warns of what JSR-305's nicknames say by default.
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import javax.annotation.Nonnull;
import javax.annotation.meta.TypeQualifierNickname;
import javax.annotation.meta.When;

@Retention(RetentionPolicy.RUNTIME)
@Nonnull(when = When.MAYBE)
@TypeQualifierNickname
public @interface MaybeNull {}
