package a

import android.app.Application
import androidx.activity.ComponentActivity
import androidx.lifecycle.ViewModel
import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.android.AndroidEntryPoint
import dagger.hilt.android.HiltAndroidApp
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.components.SingletonComponent
import javax.inject.Inject

@HiltAndroidApp
class App : Application()

class Foo @Inject constructor()

interface Bar

class BarImpl @Inject constructor() : Bar

@Module
@InstallIn(SingletonComponent::class)
internal abstract class BarModule {
    @Binds
    abstract fun bar(impl: BarImpl): Bar
}

@AndroidEntryPoint
class Screen : ComponentActivity() {
    @Inject
    lateinit var foo: Foo
}

@HiltViewModel
class ScreenModel @Inject constructor(private val bar: Bar) : ViewModel()

class Flow {
    @HiltViewModel
    class StepModel @Inject constructor() : ViewModel()
}
