package com.kyant.backdrop.music

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.media.MediaPlayer
import android.media.audiofx.Equalizer
import android.os.Handler
import android.os.Looper
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlin.math.roundToInt
import kotlin.random.Random

/** Cómo se comporta la cola al terminar una pista. */
enum class RepeatMode(val label: String) {
    Off("Sin repetición"),
    All("Repetir todo"),
    One("Repetir una pista"),
}

/** Banda del ecualizador real del dispositivo, en milibelios. */
data class EqBand(
    val index: Int,
    val label: String,
    val minMillibels: Short,
    val maxMillibels: Short,
    val millibels: Short,
) {
    val levelDb: Float get() = millibels / 100f
    val minDb: Float get() = minMillibels / 100f
    val maxDb: Float get() = maxMillibels / 100f
}

/**
 * Reproduce audio real de `MediaStore` con [MediaPlayer], pide foco de audio y
 * mantiene el estado que consume Compose.
 *
 * Es un [ViewModel] para que la reproducción sobreviva a la rotación y a los
 * cambios de configuración.
 */
class PlayerController(context: Context) : ViewModel() {

    private val appContext = context.applicationContext
    private val audioManager =
        appContext.getSystemService(Context.AUDIO_SERVICE) as AudioManager
    private val mainHandler = Handler(Looper.getMainLooper())
    private val attributes = AudioAttributes.Builder()
        .setUsage(AudioAttributes.USAGE_MEDIA)
        .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC)
        .build()

    /** True si la pausa la provocó una pérdida temporal de foco de audio. */
    private var pausedForFocusLoss = false

    /** Se puede desactivar desde Configuración para seguir sonando al bajar el volumen de otra app. */
    private var pauseOnFocusLoss = true

    private var equalizer: Equalizer? = null
    private var savedEqualizerLevels: Map<Int, Short> = emptyMap()

    private val focusRequest = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
        .setAudioAttributes(attributes)
        .setAcceptsDelayedFocusGain(false)
        .setOnAudioFocusChangeListener { change ->
            when (change) {
                AudioManager.AUDIOFOCUS_LOSS -> {
                    pausedForFocusLoss = false
                    pausePlayback(abandonFocus = false)
                }

                AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> if (pauseOnFocusLoss) {
                    pausedForFocusLoss = isPlaying
                    pausePlayback(abandonFocus = false)
                }

                AudioManager.AUDIOFOCUS_GAIN -> if (pausedForFocusLoss) {
                    pausedForFocusLoss = false
                    resumeFromFocusGain()
                }
            }
        }
        .build()

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var player: MediaPlayer? = null

    var queue by mutableStateOf<List<Track>>(emptyList())
        private set
    var currentIndex by mutableIntStateOf(-1)
        private set
    var isPlaying by mutableStateOf(false)
        private set
    var isPreparing by mutableStateOf(false)
        private set
    var positionMs by mutableLongStateOf(0L)
        private set
    var durationMs by mutableLongStateOf(0L)
        private set
    var error by mutableStateOf<String?>(null)
        private set

    /** Aleatorio activado por el usuario; se conserva al recargar la biblioteca. */
    var shuffle by mutableStateOf(false)
        private set

    var repeatMode by mutableStateOf(RepeatMode.Off)
        private set

    var eqBands by mutableStateOf<List<EqBand>>(emptyList())
        private set
    var eqError by mutableStateOf<String?>(null)
        private set

    val hasEqualizer: Boolean get() = eqBands.isNotEmpty()

    val currentTrack: Track? get() = queue.getOrNull(currentIndex)

    val progress: Float
        get() = if (durationMs <= 0L) 0f
        else (positionMs.toFloat() / durationMs).coerceIn(0f, 1f)

    init {
        scope.launch {
            while (isActive) {
                val mediaPlayer = player
                if (mediaPlayer != null && isPlaying) {
                    runCatching { positionMs = mediaPlayer.currentPosition.toLong() }
                }
                delay(250)
            }
        }
    }

    /**
     * Sustituye la cola conservando la pista actual si sigue existiendo. Al
     * recargar la biblioteca no se corta la reproducción en curso.
     */
    fun updateQueue(tracks: List<Track>) {
        val currentId = currentTrack?.id
        queue = tracks

        val preservedIndex = tracks.indexOfFirst { it.id == currentId }
        if (preservedIndex >= 0) {
            currentIndex = preservedIndex
            if (currentTrack != null && durationMs <= 0L) {
                durationMs = currentTrack?.durationMs ?: 0L
            }
            return
        }

        releasePlayer()
        currentIndex = if (tracks.isNotEmpty()) 0 else -1
        positionMs = 0L
        durationMs = tracks.firstOrNull()?.durationMs ?: 0L
        error = null
    }

    /** Reproduce la pista [index] de la cola desde el principio. */
    fun playAt(index: Int, autoPlay: Boolean = true) {
        val track = queue.getOrNull(index) ?: return
        releasePlayer()

        currentIndex = index
        positionMs = 0L
        durationMs = track.durationMs
        error = null
        isPreparing = true

        val mediaPlayer = MediaPlayer()
        player = mediaPlayer
        try {
            mediaPlayer.setAudioAttributes(attributes)
            mediaPlayer.setDataSource(appContext, track.uri)
            mediaPlayer.setOnPreparedListener { prepared ->
                isPreparing = false
                val reportedDuration = prepared.duration.toLong()
                if (reportedDuration > 0L) durationMs = reportedDuration
                attachEqualizer(prepared.audioSessionId)
                if (autoPlay) {
                    if (requestFocus()) {
                        runCatching { prepared.start() }
                            .onSuccess { isPlaying = true }
                            .onFailure {
                                isPlaying = false
                                error = "No se pudo iniciar «${track.title}»."
                            }
                    } else {
                        isPlaying = false
                        error = "Otra aplicación está usando el audio en este momento."
                    }
                } else {
                    isPlaying = false
                }
            }
            mediaPlayer.setOnCompletionListener {
                isPlaying = false
                // Se difiere el cambio de pista para no liberar el reproductor
                // dentro de su propio callback de finalización.
                mainHandler.post { advanceFromCompletion() }
            }
            mediaPlayer.setOnErrorListener { _, what, extra ->
                isPreparing = false
                isPlaying = false
                error = "No se pudo reproducir «${track.title}» (código $what/$extra)."
                true
            }
            mediaPlayer.prepareAsync()
        } catch (exception: Exception) {
            isPreparing = false
            isPlaying = false
            error = "No se pudo abrir «${track.title}»: " +
                (exception.message ?: "archivo no disponible")
            releasePlayer()
        }
    }

    /** Reproduce una pista concreta aunque la lista mostrada esté filtrada. */
    fun play(track: Track) {
        val index = queue.indexOfFirst { it.id == track.id }
        if (index >= 0) playAt(index)
    }

    fun toggle() {
        if (queue.isEmpty() || isPreparing) return
        val mediaPlayer = player
        if (mediaPlayer == null) {
            playAt(currentIndex.coerceAtLeast(0))
            return
        }
        if (isPlaying) {
            pausePlayback(abandonFocus = true)
            return
        }
        if (!requestFocus()) {
            error = "Otra aplicación está usando el audio en este momento."
            return
        }
        runCatching { mediaPlayer.start() }
            .onSuccess { isPlaying = true }
            .onFailure { error = "No se pudo reanudar la reproducción." }
    }

    fun next(autoPlay: Boolean = isPlaying) {
        if (queue.isEmpty()) return
        val nextIndex = nextIndexOrNull(wrap = true) ?: return
        playAt(nextIndex, autoPlay = autoPlay)
    }

    fun previous() {
        if (queue.isEmpty()) return
        if (positionMs > 3_000L) {
            seekTo(0L)
            return
        }
        val previousIndex = if (currentIndex <= 0) queue.lastIndex else currentIndex - 1
        playAt(previousIndex, autoPlay = isPlaying)
    }

    fun seekTo(milliseconds: Long) {
        val mediaPlayer = player
        if (mediaPlayer == null) return
        val clamped = milliseconds.coerceIn(0L, durationMs.coerceAtLeast(0L))
        runCatching { mediaPlayer.seekTo(clamped.toInt()) }
        positionMs = clamped
    }

    fun toggleShuffle() {
        shuffle = !shuffle
    }

    fun updateShuffle(value: Boolean) {
        shuffle = value
    }

    fun cycleRepeatMode() {
        repeatMode = when (repeatMode) {
            RepeatMode.Off -> RepeatMode.All
            RepeatMode.All -> RepeatMode.One
            RepeatMode.One -> RepeatMode.Off
        }
    }

    fun updateRepeatMode(value: RepeatMode) {
        repeatMode = value
    }

    fun updatePauseOnFocusLoss(value: Boolean) {
        pauseOnFocusLoss = value
    }

    /** Arranca la cola solo si no hay nada cargado; se usa al abrir la app. */
    fun playIfIdle(index: Int = currentIndex.coerceAtLeast(0)) {
        if (queue.isEmpty() || player != null || isPreparing) return
        playAt(index)
    }

    /** Cambia una banda del ecualizador real del dispositivo. */
    fun setBandLevel(index: Int, millibels: Float) {
        val effect = equalizer ?: return
        val band = eqBands.getOrNull(index) ?: return
        val clamped = millibels.roundToInt().toShort()
            .coerceIn(band.minMillibels, band.maxMillibels)
        runCatching { effect.setBandLevel(index.toShort(), clamped) }
        savedEqualizerLevels = savedEqualizerLevels + (index to clamped)
        eqBands = eqBands.toMutableList().also { bands ->
            bands[index] = band.copy(millibels = clamped)
        }
    }

    /** Devuelve todas las bandas a su ganancia neutra. */
    fun resetEqualizer() {
        if (equalizer == null) return
        eqBands.indices.forEach { index -> setBandLevel(index, 0f) }
    }

    fun clearError() {
        error = null
    }

    override fun onCleared() {
        scope.cancel()
        mainHandler.removeCallbacksAndMessages(null)
        releasePlayer()
        abandonFocus()
    }

    /**
     * Se llama al terminar una pista. Con repetición individual se reinicia la
     * misma; en el último tema sin repetición la reproducción se detiene.
     */
    private fun advanceFromCompletion() {
        if (repeatMode == RepeatMode.One) {
            playAt(currentIndex, autoPlay = true)
            return
        }

        val nextIndex = nextIndexOrNull(wrap = repeatMode == RepeatMode.All)
        if (nextIndex == null) {
            positionMs = durationMs
            return
        }
        playAt(nextIndex, autoPlay = true)
    }

    /**
     * Índice siguiente. [wrap] permite volver al principio; con aleatorio se
     * elige otra pista distinta para que no se repita la misma dos veces.
     */
    private fun nextIndexOrNull(wrap: Boolean): Int? {
        if (queue.isEmpty()) return null
        if (queue.size == 1) return if (wrap) currentIndex else null

        if (shuffle) {
            var candidate: Int
            do {
                candidate = Random.nextInt(queue.size)
            } while (candidate == currentIndex)
            return candidate
        }

        val next = currentIndex + 1
        return when {
            next < queue.size -> next
            wrap -> 0
            else -> null
        }
    }

    private fun pausePlayback(abandonFocus: Boolean) {
        val mediaPlayer = player ?: return
        runCatching {
            if (mediaPlayer.isPlaying) mediaPlayer.pause()
            positionMs = mediaPlayer.currentPosition.toLong()
        }
        isPlaying = false
        if (abandonFocus) {
            pausedForFocusLoss = false
            abandonFocus()
        }
    }

    private fun resumeFromFocusGain() {
        val mediaPlayer = player ?: return
        runCatching { mediaPlayer.start() }
            .onSuccess { isPlaying = true }
            .onFailure { error = "No se pudo reanudar la reproducción." }
    }

    private fun releasePlayer() {
        releaseEqualizer()
        player?.let { mediaPlayer ->
            runCatching { mediaPlayer.reset() }
            runCatching { mediaPlayer.release() }
        }
        player = null
        isPlaying = false
        isPreparing = false
    }

    /**
     * Crea el ecualizador sobre la sesión de audio del reproductor y recupera
     * los niveles que el usuario ya había elegido.
     */
    private fun attachEqualizer(sessionId: Int) {
        releaseEqualizer()
        eqError = null
        if (sessionId == 0) {
            eqBands = emptyList()
            return
        }
        runCatching { Equalizer(0, sessionId) }
            .onSuccess { effect ->
                equalizer = effect
                runCatching {
                    val range = effect.bandLevelRange
                    val count = effect.numberOfBands.toInt()
                    eqBands = (0 until count).map { index ->
                        val band = index.toShort()
                        savedEqualizerLevels[index]?.let { saved ->
                            runCatching { effect.setBandLevel(band, saved) }
                        }
                        EqBand(
                            index = index,
                            label = formatFrequency(effect.getCenterFreq(band)),
                            minMillibels = range[0],
                            maxMillibels = range[1],
                            millibels = runCatching { effect.getBandLevel(band) }
                                .getOrDefault(0),
                        )
                    }
                }.onFailure { failure ->
                    eqError = failure.message ?: "El ecualizador no respondió."
                    releaseEqualizer()
                }
            }
            .onFailure {
                eqError = "Este dispositivo no permite ecualizar el audio."
                eqBands = emptyList()
            }
    }

    private fun releaseEqualizer() {
        runCatching { equalizer?.release() }
        equalizer = null
        eqBands = emptyList()
    }

    private fun formatFrequency(milliHertz: Int): String {
        val hertz = (milliHertz / 1000).coerceAtLeast(0)
        return if (hertz >= 1000) {
            val whole = hertz / 1000
            val tenths = (hertz % 1000) / 100
            if (tenths == 0) "$whole kHz" else "$whole,$tenths kHz"
        } else {
            "$hertz Hz"
        }
    }

    private fun requestFocus(): Boolean {
        return audioManager.requestAudioFocus(focusRequest) ==
            AudioManager.AUDIOFOCUS_REQUEST_GRANTED
    }

    private fun abandonFocus() {
        runCatching { audioManager.abandonAudioFocusRequest(focusRequest) }
    }
}
