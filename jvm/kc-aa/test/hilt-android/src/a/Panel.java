package a;

import androidx.activity.ComponentActivity;
import dagger.hilt.android.AndroidEntryPoint;
import javax.inject.Inject;

@AndroidEntryPoint
public class Panel extends ComponentActivity {
    @Inject
    Bar bar;
}
