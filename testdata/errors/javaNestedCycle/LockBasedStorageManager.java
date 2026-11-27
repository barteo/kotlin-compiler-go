public class LockBasedStorageManager implements StorageManager {
    public interface ExceptionHandlingStrategy {
        void bar();
    }

    @Override
    public void foo() {}
}
