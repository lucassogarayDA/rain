package com.kyant.backdrop.music

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/** Orden aplicado a la cola y a la biblioteca. */
enum class LibrarySort(val label: String) {
    Title("Título"),
    Artist("Artista"),
    Album("Álbum"),
    Recent("Recientes"),
}

fun List<Track>.sortedFor(sort: LibrarySort): List<Track> = when (sort) {
    LibrarySort.Title -> sortedBy { it.title.lowercase() }
    LibrarySort.Artist -> sortedWith(
        compareBy<Track> { it.artist.lowercase() }
            .thenBy { it.album.lowercase() }
            .thenBy { it.title.lowercase() },
    )

    LibrarySort.Album -> sortedWith(
        compareBy<Track> { it.album.lowercase() }
            .thenBy { it.title.lowercase() },
    )

    LibrarySort.Recent -> sortedByDescending { it.dateAdded }
}

/**
 * Preferencias reales de la app. Cada propiedad es estado de Compose y se
 * persiste en `SharedPreferences` para sobrevivir al cierre.
 */
class MusicSettings(context: Context) {

    private val prefs = context.getSharedPreferences("backdrop-music", Context.MODE_PRIVATE)

    var autoPlay by mutableStateOf(prefs.getBoolean(KEY_AUTO_PLAY, false))
        private set

    var pauseOnFocusLoss by mutableStateOf(prefs.getBoolean(KEY_PAUSE_ON_FOCUS_LOSS, true))
        private set

    var dynamicAccent by mutableStateOf(prefs.getBoolean(KEY_DYNAMIC_ACCENT, true))
        private set

    var reduceMotion by mutableStateOf(prefs.getBoolean(KEY_REDUCE_MOTION, false))
        private set

    var sort by mutableStateOf(readSort())
        private set

    var shuffle by mutableStateOf(prefs.getBoolean(KEY_SHUFFLE, false))
        private set

    var repeatMode by mutableStateOf(readRepeatMode())
        private set

    fun updateAutoPlay(value: Boolean) {
        autoPlay = value
        prefs.edit().putBoolean(KEY_AUTO_PLAY, value).apply()
    }

    fun updatePauseOnFocusLoss(value: Boolean) {
        pauseOnFocusLoss = value
        prefs.edit().putBoolean(KEY_PAUSE_ON_FOCUS_LOSS, value).apply()
    }

    fun updateDynamicAccent(value: Boolean) {
        dynamicAccent = value
        prefs.edit().putBoolean(KEY_DYNAMIC_ACCENT, value).apply()
    }

    fun updateReduceMotion(value: Boolean) {
        reduceMotion = value
        prefs.edit().putBoolean(KEY_REDUCE_MOTION, value).apply()
    }

    fun updateSort(value: LibrarySort) {
        sort = value
        prefs.edit().putString(KEY_SORT, value.name).apply()
    }

    fun updateShuffle(value: Boolean) {
        shuffle = value
        prefs.edit().putBoolean(KEY_SHUFFLE, value).apply()
    }

    fun updateRepeatMode(value: RepeatMode) {
        repeatMode = value
        prefs.edit().putString(KEY_REPEAT, value.name).apply()
    }

    fun reset() {
        updateAutoPlay(false)
        updatePauseOnFocusLoss(true)
        updateDynamicAccent(true)
        updateReduceMotion(false)
        updateSort(LibrarySort.Title)
        updateShuffle(false)
        updateRepeatMode(RepeatMode.Off)
    }

    private fun readSort(): LibrarySort {
        val stored = prefs.getString(KEY_SORT, LibrarySort.Title.name)
        return runCatching { LibrarySort.valueOf(stored.orEmpty()) }
            .getOrDefault(LibrarySort.Title)
    }

    private fun readRepeatMode(): RepeatMode {
        val stored = prefs.getString(KEY_REPEAT, RepeatMode.Off.name)
        return runCatching { RepeatMode.valueOf(stored.orEmpty()) }
            .getOrDefault(RepeatMode.Off)
    }

    private companion object {
        const val KEY_AUTO_PLAY = "autoPlay"
        const val KEY_PAUSE_ON_FOCUS_LOSS = "pauseOnFocusLoss"
        const val KEY_DYNAMIC_ACCENT = "dynamicAccent"
        const val KEY_REDUCE_MOTION = "reduceMotion"
        const val KEY_SORT = "sort"
        const val KEY_SHUFFLE = "shuffle"
        const val KEY_REPEAT = "repeatMode"
    }
}
