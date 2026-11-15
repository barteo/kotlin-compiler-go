import java.lang.annotation.*;

@Repeatable(JS.Container.class)
@Retention(RetentionPolicy.SOURCE)
public @interface JS {
    public @interface Container {
        JS[] value();
    }
}
