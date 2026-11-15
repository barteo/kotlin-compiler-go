import java.lang.annotation.*;

@Repeatable(JR.Container.class)
@Retention(RetentionPolicy.RUNTIME)
public @interface JR {
    public @interface Container {
        JR[] value();
    }
}
