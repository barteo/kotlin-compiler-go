import java.util.AbstractMap;
import java.util.Map;
import java.util.Set;
import kotlin.jvm.PurelyImplements;

@PurelyImplements("kotlin.collections.MutableList")
public class W<K, V> extends AbstractMap<K, V> {
    public Set<Map.Entry<K, V>> entrySet() { return null; }
}
