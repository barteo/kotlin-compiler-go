import java.util.Collection;
import java.util.List;

public class JCfg implements Cfg {
    @Override public List<String> items() { return List.of(); }
    @Override public Collection<String> all() { return List.of(); }
    @Override public List<String> names() { return new java.util.ArrayList<>(); }
}
