package com.kyant.backdrop.music

import android.content.ContentUris
import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.media.MediaMetadataRetriever
import android.net.Uri
import android.os.Build
import android.provider.MediaStore
import android.util.Size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import androidx.compose.runtime.produceState
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.lerp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.util.LinkedHashMap
import kotlin.math.max

/**
 * Una pista real de `MediaStore`. [uri] es la URI de contenido que se pasa a
 * `MediaPlayer`; no se guarda ninguna ruta de archivo salvo la carpeta, que se
 * usa para agrupar la biblioteca.
 */
data class Track(
    val id: Long,
    val uri: Uri,
    val title: String,
    val artist: String,
    val album: String,
    val albumId: Long,
    val genre: String,
    val folder: String,
    val durationMs: Long,
    val sizeBytes: Long,
    val dateAdded: Long,
) {
    val durationLabel: String get() = formatDuration(durationMs)

    val folderName: String get() = folder.substringAfterLast('/').ifBlank { folder }

    /** Búsqueda tolerante a mayúsculas y acentos de la biblioteca. */
    fun matches(query: String): Boolean {
        if (query.isBlank()) return true
        val term = query.trim()
        return title.contains(term, ignoreCase = true) ||
            artist.contains(term, ignoreCase = true) ||
            album.contains(term, ignoreCase = true) ||
            genre.contains(term, ignoreCase = true) ||
            folder.contains(term, ignoreCase = true)
    }
}

/** Formatea milisegundos como m:ss sin depender de la configuración regional. */
fun formatDuration(durationMs: Long): String {
    val totalSeconds = (durationMs / 1000).coerceAtLeast(0)
    val minutes = totalSeconds / 60
    val seconds = totalSeconds % 60
    return "$minutes:${seconds.toString().padStart(2, '0')}"
}

object MusicLibrary {

    private val projection = buildList {
        add(MediaStore.Audio.Media._ID)
        add(MediaStore.Audio.Media.TITLE)
        add(MediaStore.Audio.Media.ARTIST)
        add(MediaStore.Audio.Media.ALBUM)
        add(MediaStore.Audio.Media.ALBUM_ID)
        add(MediaStore.Audio.Media.DURATION)
        add(MediaStore.Audio.Media.DATE_ADDED)
        add(MediaStore.Audio.Media.SIZE)
        add(MediaStore.Audio.Media.DATA)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            add(MediaStore.Audio.Media.GENRE)
        }
    }.toTypedArray()

    /**
     * Consulta el audio local. Solo se piden canciones de más de 15 s para
     * dejar fuera tonos y avisos del sistema.
     */
    fun load(context: Context): List<Track> {
        val collection = MediaStore.Audio.Media.EXTERNAL_CONTENT_URI
        val selection = buildString {
            append(MediaStore.Audio.Media.IS_MUSIC)
            append(" != 0 AND ")
            append(MediaStore.Audio.Media.DURATION)
            append(" >= 15000")
        }
        val sortOrder = "${MediaStore.Audio.Media.TITLE} COLLATE NOCASE ASC"
        val tracks = mutableListOf<Track>()

        context.contentResolver.query(collection, projection, selection, null, sortOrder)?.use { cursor ->
            val idColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media._ID)
            val titleColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.TITLE)
            val artistColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.ARTIST)
            val albumColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.ALBUM)
            val albumIdColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.ALBUM_ID)
            val durationColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.DURATION)
            val dateColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.DATE_ADDED)
            val sizeColumn = cursor.getColumnIndexOrThrow(MediaStore.Audio.Media.SIZE)
            val dataColumn = cursor.getColumnIndex(MediaStore.Audio.Media.DATA)
            val genreColumn = cursor.getColumnIndex(MediaStore.Audio.Media.GENRE)

            val folders = mutableMapOf<String, String>()

            while (cursor.moveToNext()) {
                val id = cursor.getLong(idColumn)
                val data = if (dataColumn >= 0) cursor.getString(dataColumn) else null
                val folder = folders.getOrPut(data.orEmpty()) {
                    data?.let { File(it).parent } ?: "Almacenamiento"
                }
                tracks += Track(
                    id = id,
                    uri = ContentUris.withAppendedId(collection, id),
                    title = cursor.getString(titleColumn).cleanOr("Pista sin título"),
                    artist = cursor.getString(artistColumn).cleanOr("Artista desconocido"),
                    album = cursor.getString(albumColumn).cleanOr("Álbum desconocido"),
                    albumId = cursor.getLong(albumIdColumn),
                    genre = if (genreColumn >= 0) {
                        cursor.getString(genreColumn).cleanOr("Sin género")
                    } else {
                        "Sin género"
                    },
                    folder = folder,
                    durationMs = cursor.getLong(durationColumn).coerceAtLeast(0L),
                    sizeBytes = cursor.getLong(sizeColumn).coerceAtLeast(0L),
                    dateAdded = cursor.getLong(dateColumn).coerceAtLeast(0L),
                )
            }
        }
        return tracks
    }

    /** `MediaStore` devuelve "<unknown>" cuando no hay metadatos. */
    private fun String?.cleanOr(fallback: String): String {
        if (this.isNullOrBlank() || this == MediaStore.UNKNOWN_STRING) return fallback
        return this
    }
}

/** Un álbum agrupado a partir de las pistas reales del dispositivo. */
data class AlbumGroup(
    val id: Long,
    val name: String,
    val artist: String,
    val tracks: List<Track>,
) {
    val key: String get() = "$id|$name"
    val artworkTrack: Track get() = tracks.first()
    val durationMs: Long get() = tracks.sumOf { it.durationMs }
}

/** Un artista agrupado a partir de las pistas reales. */
data class ArtistGroup(
    val name: String,
    val tracks: List<Track>,
) {
    val albumCount: Int get() = tracks.map { it.albumId to it.album }.distinct().size
    val durationMs: Long get() = tracks.sumOf { it.durationMs }
}

data class GenreGroup(
    val name: String,
    val tracks: List<Track>,
) {
    val durationMs: Long get() = tracks.sumOf { it.durationMs }
}

data class FolderGroup(
    val path: String,
    val name: String,
    val tracks: List<Track>,
) {
    val durationMs: Long get() = tracks.sumOf { it.durationMs }
}

object TrackGroups {

    fun albums(tracks: List<Track>): List<AlbumGroup> {
        return tracks
            .groupBy { if (it.albumId != 0L) "id:${it.albumId}" else "name:${it.album}" }
            .values
            .map { albumTracks ->
                val first = albumTracks.first()
                AlbumGroup(
                    id = first.albumId,
                    name = first.album,
                    artist = albumTracks.map { it.artist }.distinct().joinToString(" · "),
                    tracks = albumTracks,
                )
            }
            .sortedBy { it.name.lowercase() }
    }

    fun artists(tracks: List<Track>): List<ArtistGroup> {
        return tracks
            .groupBy { it.artist }
            .map { (name, artistTracks) -> ArtistGroup(name, artistTracks) }
            .sortedBy { it.name.lowercase() }
    }

    fun genres(tracks: List<Track>): List<GenreGroup> {
        return tracks
            .groupBy { it.genre }
            .map { (name, genreTracks) -> GenreGroup(name, genreTracks) }
            .sortedBy { it.name.lowercase() }
    }

    fun folders(tracks: List<Track>): List<FolderGroup> {
        return tracks
            .groupBy { it.folder }
            .map { (path, folderTracks) ->
                FolderGroup(
                    path = path,
                    name = folderTracks.first().folderName,
                    tracks = folderTracks,
                )
            }
            .sortedBy { it.name.lowercase() }
    }
}

/**
 * Carátula decodificada más su color dominante. El color se usa para animar la
 * escena cada vez que cambia la pista, sin añadir una dependencia de paletas.
 */
data class Artwork(
    val image: ImageBitmap,
    val accent: Color,
)

object ArtworkLoader {

    private const val MAX_CACHE_ENTRIES = 32

    /**
     * Caché en memoria con orden de acceso. Evita volver a decodificar la misma
     * portada al desplazarse por Inicio o la biblioteca.
     */
    private val cache = object : LinkedHashMap<String, Artwork?>(24, 0.75f, true) {
        override fun removeEldestEntry(
            eldest: MutableMap.MutableEntry<String, Artwork?>?,
        ): Boolean = size > MAX_CACHE_ENTRIES
    }

    fun load(context: Context, uri: Uri, cacheKey: String = uri.toString()): Artwork? {
        synchronized(cache) {
            if (cache.containsKey(cacheKey)) return cache[cacheKey]
        }
        val artwork = decode(context, uri)
        synchronized(cache) { cache[cacheKey] = artwork }
        return artwork
    }

    fun clearCache() {
        synchronized(cache) { cache.clear() }
    }

    private fun decode(context: Context, uri: Uri): Artwork? {
        // Android 10+ sabe extraer la miniatura incrustada sin abrir el archivo
        // entero, incluso con almacenamiento limitado.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            runCatching {
                context.contentResolver.loadThumbnail(uri, Size(384, 384), null)
            }.getOrNull()?.let { bitmap ->
                return Artwork(
                    image = bitmap.asImageBitmap(),
                    accent = dominantColor(bitmap),
                )
            }
        }

        val retriever = MediaMetadataRetriever()
        return try {
            retriever.setDataSource(context, uri)
            val bytes = retriever.embeddedPicture ?: return null
            var bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.size) ?: return null
            val longestSide = max(bitmap.width, bitmap.height)
            if (longestSide > 512) {
                val scale = 512f / longestSide
                bitmap = Bitmap.createScaledBitmap(
                    bitmap,
                    (bitmap.width * scale).toInt().coerceAtLeast(1),
                    (bitmap.height * scale).toInt().coerceAtLeast(1),
                    true,
                )
            }
            Artwork(
                image = bitmap.asImageBitmap(),
                accent = dominantColor(bitmap),
            )
        } catch (_: Exception) {
            null
        } finally {
            runCatching { retriever.release() }
        }
    }

    /**
     * Media ponderada de una miniatura. Se descartan los píxeles casi negros,
     * casi blancos y transparentes, que ensucian la media y apagan el acento.
     */
    private fun dominantColor(bitmap: Bitmap): Color {
        val sample = runCatching {
            Bitmap.createScaledBitmap(bitmap, 24, 24, true)
        }.getOrNull() ?: return DefaultAccent

        var red = 0.0
        var green = 0.0
        var blue = 0.0
        var weight = 0.0

        for (y in 0 until sample.height) {
            for (x in 0 until sample.width) {
                val pixel = sample.getPixel(x, y)
                val alpha = android.graphics.Color.alpha(pixel) / 255f
                if (alpha < 0.5f) continue
                val r = android.graphics.Color.red(pixel) / 255f
                val g = android.graphics.Color.green(pixel) / 255f
                val b = android.graphics.Color.blue(pixel) / 255f
                val luminance = 0.2126f * r + 0.7152f * g + 0.0722f * b
                if (luminance < 0.06f || luminance > 0.94f) continue
                val maxChannel = max(r, max(g, b))
                val minChannel = minOf(r, g, b)
                val saturation = if (maxChannel == 0f) 0f else (maxChannel - minChannel) / maxChannel
                val pixelWeight = 0.4 + saturation
                red += r * pixelWeight
                green += g * pixelWeight
                blue += b * pixelWeight
                weight += pixelWeight
            }
        }

        if (weight <= 0.0) return DefaultAccent
        val average = Color(
            (red / weight).toFloat(),
            (green / weight).toFloat(),
            (blue / weight).toFloat(),
        )

        // Se aclara un poco para que el acento se note en el cristal, sin
        // convertirlo en un color fluorescente.
        return lerp(average, Color.White, 0.08f).copy(alpha = 1f)
    }
}

/** Carga la carátula de una pista fuera del hilo principal y la cachea. */
@Composable
fun rememberArtwork(context: Context, track: Track?): State<Artwork?> {
    return produceState<Artwork?>(
        initialValue = null,
        key1 = track?.id,
    ) {
        val current = track
        value = if (current == null) {
            null
        } else {
            withContext(Dispatchers.IO) {
                ArtworkLoader.load(context, current.uri, "track:${current.id}")
            }
        }
    }
}

/** Carátula compartida por todas las pistas de un mismo álbum. */
@Composable
fun rememberAlbumArtwork(context: Context, album: AlbumGroup?): State<Artwork?> {
    return produceState<Artwork?>(
        initialValue = null,
        key1 = album?.key,
    ) {
        val current = album
        value = if (current == null) {
            null
        } else {
            withContext(Dispatchers.IO) {
                ArtworkLoader.load(
                    context = context,
                    uri = current.artworkTrack.uri,
                    cacheKey = "album:${current.key}",
                )
            }
        }
    }
}

/** Acento de reserva cuando la pista no trae carátula. */
val DefaultAccent: Color = Color(0xFF8B8CF8)
