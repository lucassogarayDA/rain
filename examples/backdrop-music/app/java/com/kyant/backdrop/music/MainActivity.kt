package com.kyant.backdrop.music

import android.Manifest
import android.app.Application
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.LibraryMusic
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.view.WindowCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import com.kyant.backdrop.backdrops.layerBackdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlin.random.Random

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // La escena ocupa toda la pantalla; las barras del sistema quedan
        // transparentes y los iconos en claro sobre el fondo oscuro.
        WindowCompat.setDecorFitsSystemWindows(window, false)
        window.statusBarColor = android.graphics.Color.TRANSPARENT
        window.navigationBarColor = android.graphics.Color.TRANSPARENT
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            window.isNavigationBarContrastEnforced = false
        }
        WindowCompat.getInsetsController(window, window.decorView).apply {
            isAppearanceLightStatusBars = false
            isAppearanceLightNavigationBars = false
        }

        val controller = ViewModelProvider(
            this,
            PlayerControllerFactory(application),
        )[PlayerController::class.java]

        setContent {
            BackdropMusicApp(controller)
        }
    }
}

private class PlayerControllerFactory(
    private val application: Application,
) : ViewModelProvider.Factory {
    @Suppress("UNCHECKED_CAST")
    override fun <T : ViewModel> create(modelClass: Class<T>): T {
        return PlayerController(application) as T
    }
}

private const val TAB_HOME = 0
private const val TAB_LIBRARY = 1
private const val TAB_PLAYER = 2
private const val TAB_SETTINGS = 3

@Composable
fun BackdropMusicApp(controller: PlayerController) {
    val context = LocalContext.current
    val settings = remember { MusicSettings(context) }
    val permission = remember { requiredAudioPermission() }

    var granted by remember { mutableStateOf(hasAudioPermission(context)) }
    var tracks by remember { mutableStateOf<List<Track>>(emptyList()) }
    var loading by remember { mutableStateOf(false) }
    var loadError by remember { mutableStateOf<String?>(null) }
    var reloadToken by remember { mutableIntStateOf(0) }
    var tab by rememberSaveable { mutableIntStateOf(TAB_HOME) }
    var query by rememberSaveable { mutableStateOf("") }
    var eqRequest by remember { mutableIntStateOf(0) }

    val permissionLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.RequestPermission(),
    ) { result ->
        granted = result
        if (result) reloadToken++
    }

    // Al volver de Ajustes el permiso puede haber cambiado: se revisa en cada
    // ON_RESUME para no obligar a reiniciar la app.
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) {
                val nowGranted = hasAudioPermission(context)
                if (nowGranted != granted) {
                    granted = nowGranted
                    if (nowGranted) reloadToken++
                }
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    LaunchedEffect(granted, reloadToken) {
        if (!granted) {
            tracks = emptyList()
            loadError = null
            return@LaunchedEffect
        }

        loading = true
        loadError = null
        runCatching {
            withContext(Dispatchers.IO) { MusicLibrary.load(context) }
        }.onSuccess { loaded ->
            tracks = loaded
        }.onFailure { failure ->
            loadError = failure.message
                ?: "El sistema no devolvió la lista de audio."
        }
        loading = false
    }

    // El orden elegido en Configuración se aplica a la cola sin cortar la
    // canción que esté sonando.
    LaunchedEffect(tracks, settings.sort) {
        controller.updateQueue(tracks.sortedFor(settings.sort))
    }

    // Las preferencias y el reproductor se mantienen sincronizados en ambos
    // sentidos: los controles del reproductor también las actualizan.
    LaunchedEffect(settings.shuffle) { controller.updateShuffle(settings.shuffle) }
    LaunchedEffect(settings.repeatMode) { controller.updateRepeatMode(settings.repeatMode) }
    LaunchedEffect(settings.pauseOnFocusLoss) {
        controller.updatePauseOnFocusLoss(settings.pauseOnFocusLoss)
    }

    LaunchedEffect(controller.queue, settings.autoPlay) {
        if (settings.autoPlay && controller.queue.isNotEmpty()) {
            controller.playIfIdle()
        }
    }

    // La carátula se decodifica solo para la pista actual; al cambiar de tema
    // se mantiene la anterior hasta que la nueva está lista y luego se funde.
    val artwork by produceState<Artwork?>(
        initialValue = null,
        key1 = controller.currentTrack?.id,
    ) {
        val track = controller.currentTrack
        value = if (track == null) {
            null
        } else {
            withContext(Dispatchers.IO) {
                ArtworkLoader.load(context, track.uri, "track:${track.id}")
            }
        }
    }

    val accent by animateColorAsState(
        targetValue = if (settings.dynamicAccent) {
            artwork?.accent ?: DefaultAccent
        } else {
            DefaultAccent
        },
        animationSpec = tween(durationMillis = if (settings.reduceMotion) 0 else 700),
        label = "acento",
    )

    val backdrop = rememberLayerBackdrop()
    val showChrome = granted && !loading && loadError == null && tracks.isNotEmpty()
    val current = controller.currentTrack

    BackHandler(enabled = tab != TAB_HOME || query.isNotEmpty()) {
        if (query.isNotEmpty()) query = "" else tab = TAB_HOME
    }

    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = accent,
            onPrimary = Color(0xFF0B0B12),
            background = Color(0xFF05050A),
            onBackground = Color.White,
            surface = Color(0xFF12121A),
            onSurface = Color.White,
            surfaceVariant = Color(0xFF1C1C28),
            onSurfaceVariant = Color.White.copy(alpha = 0.7f),
        ),
    ) {
        CompositionLocalProvider(LocalContentColor provides Color.White) {
            Box(modifier = Modifier.fillMaxSize()) {
                // Solo la escena se graba en el backdrop. La interfaz de cristal
                // queda fuera para que nunca se refracte a sí misma.
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .layerBackdrop(backdrop),
                ) {
                    SceneBackground(
                        artwork = artwork?.image,
                        accent = accent,
                        reduceMotion = settings.reduceMotion,
                    )
                }

                Scaffold(
                    modifier = Modifier.fillMaxSize(),
                    containerColor = Color.Transparent,
                    contentWindowInsets = WindowInsets(0, 0, 0, 0),
                    topBar = {
                        AppTopBar(
                            subtitle = when {
                                !granted -> "Permiso de audio pendiente"
                                loading -> "Buscando audio local…"
                                loadError != null -> "Error al leer la biblioteca"
                                tracks.isEmpty() -> "Sin pistas en el dispositivo"
                                else -> pageSubtitle(tab)
                            },
                            granted = granted,
                            loading = loading,
                            backdrop = backdrop,
                            onRefresh = {
                                if (granted) reloadToken++ else permissionLauncher.launch(permission)
                            },
                        )
                    },
                    bottomBar = {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .navigationBarsPadding(),
                        ) {
                            AnimatedVisibility(
                                visible = showChrome && tab != TAB_PLAYER && current != null,
                                enter = fadeIn(tween(220)) +
                                    expandVertically(expandFrom = Alignment.Bottom),
                                exit = fadeOut(tween(160)) +
                                    shrinkVertically(shrinkTowards = Alignment.Bottom),
                            ) {
                                current?.let { playing ->
                                    MiniPlayer(
                                        backdrop = backdrop,
                                        track = playing,
                                        artwork = artwork?.image,
                                        isPlaying = controller.isPlaying,
                                        isPreparing = controller.isPreparing,
                                        progress = controller.progress,
                                        accent = accent,
                                        onClick = { tab = TAB_PLAYER },
                                        onToggle = controller::toggle,
                                        modifier = Modifier.padding(
                                            horizontal = 16.dp,
                                            vertical = 6.dp,
                                        ),
                                    )
                                }
                            }
                            GlassNavigationBar(
                                backdrop = backdrop,
                                selected = tab,
                                onSelect = { tab = it },
                                accent = accent,
                            )
                        }
                    },
                ) { innerPadding ->
                    AnimatedContent(
                        targetState = tab,
                        transitionSpec = {
                            val duration = if (settings.reduceMotion) 0 else 380
                            val direction = if (targetState > initialState) 1 else -1
                            (
                                slideInHorizontally(
                                    animationSpec = tween(
                                        durationMillis = duration,
                                        easing = FastOutSlowInEasing,
                                    ),
                                ) { width -> direction * width / 5 } +
                                    fadeIn(tween(if (settings.reduceMotion) 0 else 300))
                                ).togetherWith(
                                slideOutHorizontally(
                                    animationSpec = tween(
                                        durationMillis = duration,
                                        easing = FastOutSlowInEasing,
                                    ),
                                ) { width -> -direction * width / 5 } +
                                    fadeOut(tween(if (settings.reduceMotion) 0 else 200)),
                            )
                        },
                        modifier = Modifier
                            .fillMaxSize()
                            .padding(innerPadding),
                        label = "páginas",
                    ) { page ->
                        when {
                            // Configuración funciona aunque falte el permiso de audio.
                            page == TAB_SETTINGS -> SettingsScreen(
                                settings = settings,
                                controller = controller,
                                tracks = tracks,
                                backdrop = backdrop,
                                accent = accent,
                                onRescan = {
                                    if (granted) reloadToken++
                                    else permissionLauncher.launch(permission)
                                },
                                onOpenEqualizer = {
                                    eqRequest++
                                    tab = TAB_PLAYER
                                },
                                modifier = Modifier.fillMaxSize(),
                            )

                            !granted -> CenteredStatus {
                                StatusCard(
                                    backdrop = backdrop,
                                    icon = Icons.Rounded.Lock,
                                    title = "Tu música, sin salir del dispositivo",
                                    message = "Backdrop Music necesita permiso para leer el audio " +
                                        "local. No se sube nada a internet: MediaStore y " +
                                        "MediaPlayer trabajan con archivos del teléfono.",
                                    primaryLabel = "Conceder permiso",
                                    onPrimary = { permissionLauncher.launch(permission) },
                                    secondaryLabel = "Abrir ajustes",
                                    onSecondary = { openAppSettings(context) },
                                )
                            }

                            loading -> CenteredStatus {
                                StatusCard(
                                    backdrop = backdrop,
                                    icon = Icons.Rounded.LibraryMusic,
                                    title = "Buscando audio local",
                                    message = "Leyendo la biblioteca de MediaStore…",
                                    showProgress = true,
                                )
                            }

                            loadError != null -> CenteredStatus {
                                StatusCard(
                                    backdrop = backdrop,
                                    icon = Icons.Rounded.LibraryMusic,
                                    title = "No se pudo leer la biblioteca",
                                    message = loadError
                                        ?: "El sistema devolvió un error al consultar MediaStore.",
                                    primaryLabel = "Reintentar",
                                    onPrimary = { reloadToken++ },
                                    secondaryLabel = "Abrir ajustes",
                                    onSecondary = { openAppSettings(context) },
                                )
                            }

                            tracks.isEmpty() -> CenteredStatus {
                                StatusCard(
                                    backdrop = backdrop,
                                    icon = Icons.Rounded.LibraryMusic,
                                    title = "No hay música todavía",
                                    message = "Añade archivos de audio al dispositivo y espera a " +
                                        "que el sistema los indexe. Después vuelve a comprobar.",
                                    primaryLabel = "Volver a comprobar",
                                    onPrimary = { reloadToken++ },
                                )
                            }

                            page == TAB_HOME -> HomeScreen(
                                controller = controller,
                                backdrop = backdrop,
                                accent = accent,
                                onOpenPlayer = { tab = TAB_PLAYER },
                                onOpenLibrary = { tab = TAB_LIBRARY },
                                onOpenAlbum = { album ->
                                    query = album
                                    tab = TAB_LIBRARY
                                },
                                onShufflePlay = {
                                    if (controller.queue.isNotEmpty()) {
                                        controller.updateShuffle(true)
                                        settings.updateShuffle(true)
                                        controller.playAt(Random.nextInt(controller.queue.size))
                                    }
                                    tab = TAB_PLAYER
                                },
                                modifier = Modifier.fillMaxSize(),
                            )

                            page == TAB_LIBRARY -> LibraryScreen(
                                controller = controller,
                                backdrop = backdrop,
                                query = query,
                                onQueryChange = { query = it },
                                accent = accent,
                                onOpenPlayer = { tab = TAB_PLAYER },
                                onToggleShuffle = {
                                    controller.toggleShuffle()
                                    settings.updateShuffle(controller.shuffle)
                                },
                                modifier = Modifier.fillMaxSize(),
                            )

                            else -> PlayerScreen(
                                controller = controller,
                                backdrop = backdrop,
                                artwork = artwork,
                                accent = accent,
                                onToggleShuffle = {
                                    controller.toggleShuffle()
                                    settings.updateShuffle(controller.shuffle)
                                },
                                onCycleRepeat = {
                                    controller.cycleRepeatMode()
                                    settings.updateRepeatMode(controller.repeatMode)
                                },
                                eqRequest = eqRequest,
                                modifier = Modifier.fillMaxSize(),
                            )
                        }
                    }
                }
            }
        }
    }
}

private fun pageSubtitle(tab: Int): String = when (tab) {
    TAB_HOME -> "Recientes, álbumes y reproducción"
    TAB_LIBRARY -> "Canciones, artistas, álbumes, géneros y carpetas"
    TAB_PLAYER -> "Portada, controles, cola y ecualizador"
    else -> "Audio, apariencia, biblioteca, reproducción y almacenamiento"
}

@Composable
private fun CenteredStatus(content: @Composable () -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 20.dp, vertical = 16.dp),
        contentAlignment = Alignment.Center,
    ) {
        content()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AppTopBar(
    subtitle: String,
    granted: Boolean,
    loading: Boolean,
    backdrop: com.kyant.backdrop.Backdrop,
    onRefresh: () -> Unit,
) {
    TopAppBar(
        title = {
            Column {
                Text(
                    text = "Backdrop Music",
                    fontSize = 21.sp,
                    fontWeight = FontWeight.Bold,
                )
                Text(
                    text = subtitle,
                    fontSize = 12.sp,
                    color = Color.White.copy(alpha = 0.62f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        },
        actions = {
            GlassIconButton(
                backdrop = backdrop,
                icon = Icons.Rounded.Refresh,
                contentDescription = if (granted) {
                    "Actualizar biblioteca"
                } else {
                    "Pedir permiso de audio"
                },
                onClick = onRefresh,
                enabled = !loading,
                buttonSize = 44.dp,
                iconSize = 20.dp,
            )
            Spacer(Modifier.width(14.dp))
        },
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = Color.Transparent,
            titleContentColor = Color.White,
            actionIconContentColor = Color.White,
        ),
    )
}

private fun requiredAudioPermission(): String =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
        Manifest.permission.READ_MEDIA_AUDIO
    } else {
        Manifest.permission.READ_EXTERNAL_STORAGE
    }

private fun hasAudioPermission(context: Context): Boolean =
    context.checkSelfPermission(requiredAudioPermission()) == PackageManager.PERMISSION_GRANTED

private fun openAppSettings(context: Context) {
    val intent = Intent(
        Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
        Uri.fromParts("package", context.packageName, null),
    ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    runCatching { context.startActivity(intent) }
}
