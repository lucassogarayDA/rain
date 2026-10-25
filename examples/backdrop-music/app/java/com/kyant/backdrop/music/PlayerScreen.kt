package com.kyant.backdrop.music

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Album
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.ErrorOutline
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material.icons.rounded.Folder
import androidx.compose.material.icons.rounded.GraphicEq
import androidx.compose.material.icons.rounded.MusicNote
import androidx.compose.material.icons.rounded.Person
import androidx.compose.material.icons.rounded.QueueMusic
import androidx.compose.material.icons.rounded.Repeat
import androidx.compose.material.icons.rounded.RepeatOne
import androidx.compose.material.icons.rounded.Shuffle
import androidx.compose.material.icons.rounded.SkipNext
import androidx.compose.material.icons.rounded.SkipPrevious
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kyant.backdrop.Backdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop

/**
 * Página del reproductor. La carátula se funde al cambiar de canción y los
 * metadatos entran deslizándose; debajo viven los controles, la cola y el
 * ecualizador real, cada uno en su panel de cristal.
 */
@Composable
fun PlayerScreen(
    controller: PlayerController,
    backdrop: Backdrop,
    artwork: Artwork?,
    accent: Color,
    modifier: Modifier = Modifier,
    onToggleShuffle: () -> Unit = controller::toggleShuffle,
    onCycleRepeat: () -> Unit = controller::cycleRepeatMode,
    eqRequest: Int = 0,
) {
    val track = controller.currentTrack ?: return
    var lastError by remember { mutableStateOf<String?>(null) }
    var queueExpanded by rememberSaveable { mutableStateOf(false) }
    var eqExpanded by rememberSaveable { mutableStateOf(false) }

    LaunchedEffect(controller.error) {
        controller.error?.let { lastError = it }
    }
    LaunchedEffect(eqRequest) {
        if (eqRequest > 0) eqExpanded = true
    }

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 6.dp, bottom = 20.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "caratula") {
            ArtworkCard(
                artwork = artwork?.image,
                accent = accent,
                isPlaying = controller.isPlaying,
                isPreparing = controller.isPreparing,
                trackTitle = track.title,
            )
        }

        item(key = "controles") {
            GlassPanel(
                backdrop = backdrop,
                modifier = Modifier.fillMaxWidth(),
                contentPadding = PaddingValues(horizontal = 18.dp, vertical = 20.dp),
            ) {
                AnimatedContent(
                    targetState = track,
                    transitionSpec = {
                        (slideInVertically(tween(320)) { it / 2 } + fadeIn(tween(280)))
                            .togetherWith(
                                slideOutVertically(tween(280)) { -it / 2 } + fadeOut(tween(180)),
                            )
                    },
                    label = "metadatos",
                ) { current ->
                    Column(modifier = Modifier.fillMaxWidth()) {
                        Text(
                            text = current.title,
                            color = Color.White,
                            fontSize = 22.sp,
                            fontWeight = FontWeight.Bold,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Spacer(Modifier.height(4.dp))
                        Text(
                            text = "${current.artist} · ${current.album}",
                            color = Color.White.copy(alpha = 0.68f),
                            fontSize = 13.sp,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }

                Spacer(Modifier.height(10.dp))

                GlassSeekBar(
                    backdrop = backdrop,
                    positionMs = controller.positionMs,
                    durationMs = controller.durationMs,
                    accent = accent,
                    onSeek = controller::seekTo,
                )

                Spacer(Modifier.height(10.dp))

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    GlassIconButton(
                        backdrop = backdrop,
                        icon = Icons.Rounded.Shuffle,
                        contentDescription = if (controller.shuffle) {
                            "Desactivar aleatorio"
                        } else {
                            "Activar aleatorio"
                        },
                        onClick = onToggleShuffle,
                        active = controller.shuffle,
                        accent = accent,
                        buttonSize = 46.dp,
                        iconSize = 21.dp,
                    )
                    GlassIconButton(
                        backdrop = backdrop,
                        icon = Icons.Rounded.SkipPrevious,
                        contentDescription = "Pista anterior",
                        onClick = controller::previous,
                        buttonSize = 54.dp,
                        iconSize = 28.dp,
                    )
                    GlassPlayButton(
                        backdrop = backdrop,
                        isPlaying = controller.isPlaying,
                        isPreparing = controller.isPreparing,
                        accent = accent,
                        onClick = controller::toggle,
                    )
                    GlassIconButton(
                        backdrop = backdrop,
                        icon = Icons.Rounded.SkipNext,
                        contentDescription = "Pista siguiente",
                        onClick = { controller.next() },
                        buttonSize = 54.dp,
                        iconSize = 28.dp,
                    )
                    GlassIconButton(
                        backdrop = backdrop,
                        icon = if (controller.repeatMode == RepeatMode.One) {
                            Icons.Rounded.RepeatOne
                        } else {
                            Icons.Rounded.Repeat
                        },
                        contentDescription = when (controller.repeatMode) {
                            RepeatMode.Off -> "Activar repetición"
                            RepeatMode.All -> "Repetir una sola pista"
                            RepeatMode.One -> "Desactivar repetición"
                        },
                        onClick = onCycleRepeat,
                        active = controller.repeatMode != RepeatMode.Off,
                        accent = accent,
                        buttonSize = 46.dp,
                        iconSize = 21.dp,
                    )
                }

                val modeLabel = buildString {
                    if (controller.shuffle) append("Aleatorio")
                    if (controller.repeatMode != RepeatMode.Off) {
                        if (isNotEmpty()) append(" · ")
                        append(controller.repeatMode.label)
                    }
                }
                if (modeLabel.isNotEmpty()) {
                    Spacer(Modifier.height(10.dp))
                    Text(
                        text = modeLabel,
                        color = accent,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Medium,
                        textAlign = TextAlign.Center,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }

                AnimatedVisibility(
                    visible = controller.error != null,
                    enter = fadeIn(tween(220)) + expandVertically(),
                    exit = fadeOut(tween(160)) + shrinkVertically(),
                ) {
                    lastError?.let { message ->
                        Spacer(Modifier.height(10.dp))
                        ErrorRow(message = message, onDismiss = controller::clearError)
                    }
                }

                Spacer(Modifier.height(10.dp))
                Text(
                    text = glassSupportLabel(),
                    color = Color.White.copy(alpha = 0.48f),
                    fontSize = 11.sp,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }

        item(key = "cola") {
            GlassPanel(
                backdrop = backdrop,
                modifier = Modifier.fillMaxWidth(),
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp),
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        imageVector = Icons.Rounded.QueueMusic,
                        contentDescription = null,
                        tint = accent,
                        modifier = Modifier.size(20.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "Cola de reproducción",
                            color = Color.White,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            text = if (controller.queue.size == 1) {
                                "1 pista"
                            } else {
                                "${controller.queue.size} pistas"
                            },
                            color = Color.White.copy(alpha = 0.55f),
                            fontSize = 11.sp,
                        )
                    }
                    GlassIconButton(
                        backdrop = backdrop,
                        icon = if (queueExpanded) {
                            Icons.Rounded.ExpandLess
                        } else {
                            Icons.Rounded.ExpandMore
                        },
                        contentDescription = if (queueExpanded) {
                            "Ocultar cola"
                        } else {
                            "Mostrar cola"
                        },
                        onClick = { queueExpanded = !queueExpanded },
                        buttonSize = 40.dp,
                        iconSize = 20.dp,
                    )
                }

                AnimatedVisibility(
                    visible = queueExpanded,
                    enter = fadeIn(tween(220)) + expandVertically(),
                    exit = fadeOut(tween(160)) + shrinkVertically(),
                ) {
                    Column(modifier = Modifier.padding(top = 8.dp)) {
                        controller.queue.take(30).forEachIndexed { index, queued ->
                            TrackRow(
                                track = queued,
                                selected = controller.currentTrack?.id == queued.id,
                                playing = controller.isPlaying &&
                                    controller.currentTrack?.id == queued.id,
                                accent = accent,
                                onClick = { controller.playAt(index) },
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                        if (controller.queue.size > 30) {
                            Text(
                                text = "y ${controller.queue.size - 30} pistas más",
                                color = Color.White.copy(alpha = 0.5f),
                                fontSize = 11.sp,
                                textAlign = TextAlign.Center,
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(vertical = 8.dp),
                            )
                        }
                    }
                }
            }
        }

        item(key = "ecualizador") {
            GlassPanel(
                backdrop = backdrop,
                modifier = Modifier.fillMaxWidth(),
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp),
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        imageVector = Icons.Rounded.GraphicEq,
                        contentDescription = null,
                        tint = accent,
                        modifier = Modifier.size(20.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "Ecualizador",
                            color = Color.White,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            text = when {
                                controller.hasEqualizer ->
                                    "${controller.eqBands.size} bandas del dispositivo"
                                controller.eqError != null -> controller.eqError.orEmpty()
                                else -> "Se activa al reproducir audio"
                            },
                            color = Color.White.copy(alpha = 0.55f),
                            fontSize = 11.sp,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    GlassIconButton(
                        backdrop = backdrop,
                        icon = if (eqExpanded) {
                            Icons.Rounded.ExpandLess
                        } else {
                            Icons.Rounded.ExpandMore
                        },
                        contentDescription = if (eqExpanded) {
                            "Ocultar ecualizador"
                        } else {
                            "Abrir ecualizador"
                        },
                        onClick = { eqExpanded = !eqExpanded },
                        buttonSize = 40.dp,
                        iconSize = 20.dp,
                    )
                }

                AnimatedVisibility(
                    visible = eqExpanded,
                    enter = fadeIn(tween(220)) + expandVertically(),
                    exit = fadeOut(tween(160)) + shrinkVertically(),
                ) {
                    Column(modifier = Modifier.padding(top = 8.dp)) {
                        if (controller.hasEqualizer) {
                            controller.eqBands.forEach { band ->
                                val minDb = band.minDb
                                val maxDb = band.maxDb.coerceAtLeast(minDb + 1f)
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .heightIn(min = 46.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Text(
                                        text = band.label,
                                        color = Color.White.copy(alpha = 0.78f),
                                        fontSize = 12.sp,
                                        modifier = Modifier.width(64.dp),
                                    )
                                    Slider(
                                        value = band.levelDb.coerceIn(minDb, maxDb),
                                        onValueChange = { value ->
                                            controller.setBandLevel(band.index, value * 100f)
                                        },
                                        valueRange = minDb..maxDb,
                                        colors = SliderDefaults.colors(
                                            thumbColor = accent,
                                            activeTrackColor = accent,
                                            inactiveTrackColor = Color.White.copy(alpha = 0.22f),
                                        ),
                                        modifier = Modifier.weight(1f),
                                    )
                                    Text(
                                        text = formatDb(band.levelDb),
                                        color = Color.White.copy(alpha = 0.6f),
                                        fontSize = 11.sp,
                                        textAlign = TextAlign.End,
                                        modifier = Modifier.width(52.dp),
                                    )
                                }
                            }
                            TextButton(
                                onClick = controller::resetEqualizer,
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Text(
                                    text = "Restablecer bandas",
                                    color = Color.White.copy(alpha = 0.75f),
                                    fontSize = 12.sp,
                                )
                            }
                        } else {
                            Text(
                                text = "Reproduce una canción para que el sistema " +
                                    "entregue su sesión de audio y aparezcan las bandas. " +
                                    "No todos los dispositivos permiten ecualizar.",
                                color = Color.White.copy(alpha = 0.62f),
                                fontSize = 12.sp,
                                modifier = Modifier.padding(vertical = 6.dp),
                            )
                        }
                    }
                }
            }
        }
    }
}

private fun formatDb(levelDb: Float): String {
    val rounded = (levelDb * 10f).toInt() / 10f
    val sign = if (rounded > 0f) "+" else ""
    return "$sign${rounded} dB"
}

@Composable
private fun ErrorRow(message: String, onDismiss: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color(0x33EF4444), RoundedCornerShape(16.dp))
            .padding(start = 12.dp, end = 4.dp, top = 8.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = Icons.Rounded.ErrorOutline,
            contentDescription = null,
            tint = Color(0xFFFFB4B4),
            modifier = Modifier.size(20.dp),
        )
        Spacer(Modifier.width(8.dp))
        Text(
            text = message,
            color = Color(0xFFFFD7D7),
            fontSize = 12.sp,
            modifier = Modifier.weight(1f),
        )
        TextButton(onClick = onDismiss) {
            Text("Cerrar", color = Color.White, fontSize = 12.sp)
        }
    }
}

/**
 * Biblioteca con las cinco vistas que pide el reproductor: canciones, artistas,
 * álbumes, géneros y carpetas. El buscador es un cristal anidado que refracta
 * el propio panel en vez del fondo.
 */
@Composable
fun LibraryScreen(
    controller: PlayerController,
    backdrop: Backdrop,
    query: String,
    onQueryChange: (String) -> Unit,
    accent: Color,
    onOpenPlayer: () -> Unit,
    modifier: Modifier = Modifier,
    onToggleShuffle: () -> Unit = controller::toggleShuffle,
) {
    val panelBackdrop = rememberLayerBackdrop()
    val tracks = controller.queue
    var category by rememberSaveable { mutableStateOf(0) }
    var groupFilter by rememberSaveable { mutableStateOf<String?>(null) }

    val filterKind = groupFilter?.substringBefore(':')
    val filterValue = groupFilter?.substringAfter(':')

    val filtered = remember(tracks, query, filterKind, filterValue) {
        tracks.filter { track ->
            track.matches(query) && when (filterKind) {
                "artist" -> track.artist == filterValue
                "album" -> track.album == filterValue
                "genre" -> track.genre == filterValue
                "folder" -> track.folder == filterValue
                else -> true
            }
        }
    }
    val albums = remember(tracks, query) {
        TrackGroups.albums(tracks).filter {
            query.isBlank() ||
                it.name.contains(query, ignoreCase = true) ||
                it.artist.contains(query, ignoreCase = true)
        }
    }
    val artists = remember(tracks, query) {
        TrackGroups.artists(tracks).filter { it.name.contains(query, ignoreCase = true) }
    }
    val genres = remember(tracks, query) {
        TrackGroups.genres(tracks).filter { it.name.contains(query, ignoreCase = true) }
    }
    val folders = remember(tracks, query) {
        TrackGroups.folders(tracks).filter { it.name.contains(query, ignoreCase = true) }
    }

    GlassPanel(
        backdrop = backdrop,
        exportedBackdrop = panelBackdrop,
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(top = 16.dp, bottom = 6.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "Biblioteca",
                        color = Color.White,
                        fontSize = 20.sp,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(
                        text = if (query.isBlank() && groupFilter == null) {
                            "${
                                when (tracks.size) {
                                    1 -> "1 pista"
                                    else -> "${tracks.size} pistas"
                                }
                            } en este dispositivo"
                        } else {
                            "${filtered.size} de ${tracks.size} pistas"
                        },
                        color = Color.White.copy(alpha = 0.6f),
                        fontSize = 12.sp,
                    )
                }
                GlassIconButton(
                    backdrop = backdrop,
                    icon = Icons.Rounded.Shuffle,
                    contentDescription = if (controller.shuffle) {
                        "Desactivar aleatorio"
                    } else {
                        "Activar aleatorio"
                    },
                    onClick = onToggleShuffle,
                    active = controller.shuffle,
                    accent = accent,
                    buttonSize = 44.dp,
                    iconSize = 20.dp,
                )
            }

            Spacer(Modifier.height(10.dp))

            GlassSearchField(
                backdrop = panelBackdrop,
                query = query,
                onQueryChange = onQueryChange,
                accent = accent,
                nested = true,
            )

            Spacer(Modifier.height(8.dp))

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                LibraryCategory.values().forEachIndexed { index, item ->
                    CategoryChip(
                        label = item.label,
                        selected = category == index,
                        accent = accent,
                        onClick = { category = index },
                    )
                }
            }

            if (groupFilter != null) {
                Spacer(Modifier.height(6.dp))
                Row(
                    modifier = Modifier
                        .clip(CircleShape)
                        .background(accent.copy(alpha = 0.28f))
                        .clickable(
                            role = Role.Button,
                            onClick = { groupFilter = null },
                        )
                        .padding(start = 12.dp, end = 8.dp, top = 5.dp, bottom = 5.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = "${filterKind.labelForFilter()}: $filterValue",
                        color = Color.White,
                        fontSize = 11.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Spacer(Modifier.width(6.dp))
                    Icon(
                        imageVector = Icons.Rounded.Close,
                        contentDescription = "Quitar filtro",
                        tint = Color.White.copy(alpha = 0.8f),
                        modifier = Modifier.size(14.dp),
                    )
                }
            }
        }

        Spacer(Modifier.height(6.dp))

        val isEmpty = when (category) {
            0 -> filtered.isEmpty()
            1 -> artists.isEmpty()
            2 -> albums.isEmpty()
            3 -> genres.isEmpty()
            else -> folders.isEmpty()
        }

        if (isEmpty) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center,
            ) {
                Icon(
                    imageVector = Icons.Rounded.MusicNote,
                    contentDescription = null,
                    tint = Color.White.copy(alpha = 0.4f),
                    modifier = Modifier.size(36.dp),
                )
                Spacer(Modifier.height(10.dp))
                Text(
                    text = if (query.isBlank() && groupFilter == null) {
                        "Esta vista está vacía"
                    } else {
                        "Sin resultados para «$query»"
                    },
                    color = Color.White.copy(alpha = 0.8f),
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    textAlign = TextAlign.Center,
                )
                Spacer(Modifier.height(4.dp))
                Text(
                    text = "Prueba con otro título, artista, álbum, género o carpeta.",
                    color = Color.White.copy(alpha = 0.55f),
                    fontSize = 12.sp,
                    textAlign = TextAlign.Center,
                )
            }
        } else {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f),
            ) {
                when (category) {
                    0 -> LazyColumn(
                        contentPadding = PaddingValues(start = 8.dp, end = 8.dp, bottom = 10.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(filtered, key = { it.id }) { track ->
                            TrackRow(
                                track = track,
                                selected = controller.currentTrack?.id == track.id,
                                playing = controller.isPlaying &&
                                    controller.currentTrack?.id == track.id,
                                accent = accent,
                                onClick = {
                                    controller.play(track)
                                    onOpenPlayer()
                                },
                                modifier = Modifier.animateItem(),
                            )
                        }
                    }

                    1 -> LazyColumn(
                        contentPadding = PaddingValues(start = 8.dp, end = 8.dp, bottom = 10.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(artists, key = { it.name }) { artist ->
                            GroupRow(
                                icon = Icons.Rounded.Person,
                                title = artist.name,
                                subtitle = "${countLabel(artist.tracks.size)} · " +
                                    "${artist.albumCount} álbumes",
                                accent = accent,
                                onClick = {
                                    groupFilter = "artist:${artist.name}"
                                    category = 0
                                },
                            )
                        }
                    }

                    2 -> LazyColumn(
                        contentPadding = PaddingValues(start = 8.dp, end = 8.dp, bottom = 10.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(albums, key = { it.key }) { album ->
                            GroupRow(
                                icon = Icons.Rounded.Album,
                                title = album.name,
                                subtitle = "${album.artist} · ${countLabel(album.tracks.size)}",
                                accent = accent,
                                onClick = {
                                    groupFilter = "album:${album.name}"
                                    category = 0
                                },
                            )
                        }
                    }

                    3 -> LazyColumn(
                        contentPadding = PaddingValues(start = 8.dp, end = 8.dp, bottom = 10.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(genres, key = { it.name }) { genre ->
                            GroupRow(
                                icon = Icons.Rounded.GraphicEq,
                                title = genre.name,
                                subtitle = countLabel(genre.tracks.size),
                                accent = accent,
                                onClick = {
                                    groupFilter = "genre:${genre.name}"
                                    category = 0
                                },
                            )
                        }
                    }

                    else -> LazyColumn(
                        contentPadding = PaddingValues(start = 8.dp, end = 8.dp, bottom = 10.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(folders, key = { it.path }) { folder ->
                            GroupRow(
                                icon = Icons.Rounded.Folder,
                                title = folder.name,
                                subtitle = "${countLabel(folder.tracks.size)} · ${folder.path}",
                                accent = accent,
                                onClick = {
                                    groupFilter = "folder:${folder.path}"
                                    category = 0
                                },
                            )
                        }
                    }
                }
            }
        }
    }
}

private enum class LibraryCategory(val label: String) {
    Songs("Canciones"),
    Artists("Artistas"),
    Albums("Álbumes"),
    Genres("Géneros"),
    Folders("Carpetas"),
}

private fun String?.labelForFilter(): String = when (this) {
    "artist" -> "Artista"
    "album" -> "Álbum"
    "genre" -> "Género"
    "folder" -> "Carpeta"
    else -> "Filtro"
}

private fun countLabel(count: Int): String = if (count == 1) "1 pista" else "$count pistas"

@Composable
private fun CategoryChip(
    label: String,
    selected: Boolean,
    accent: Color,
    onClick: () -> Unit,
) {
    val background by animateColorAsState(
        targetValue = if (selected) accent.copy(alpha = 0.42f) else Color.White.copy(alpha = 0.10f),
        animationSpec = tween(200),
        label = "chip-fondo",
    )
    val textColor by animateColorAsState(
        targetValue = if (selected) Color.White else Color.White.copy(alpha = 0.66f),
        animationSpec = tween(200),
        label = "chip-texto",
    )

    Box(
        modifier = Modifier
            .clip(CircleShape)
            .background(background)
            .clickable(role = Role.Tab, onClick = onClick)
            .padding(horizontal = 13.dp, vertical = 7.dp),
    ) {
        Text(
            text = label,
            color = textColor,
            fontSize = 12.sp,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
        )
    }
}

@Composable
private fun GroupRow(
    icon: ImageVector,
    title: String,
    subtitle: String,
    accent: Color,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 60.dp)
            .clip(RoundedCornerShape(18.dp))
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier = Modifier
                .size(44.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(
                    Brush.linearGradient(
                        colors = listOf(
                            lerp(accent, Color.Black, 0.4f),
                            Color(0xFF161626),
                        ),
                    ),
                ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = Color.White.copy(alpha = 0.85f),
                modifier = Modifier.size(20.dp),
            )
        }

        Spacer(Modifier.width(12.dp))

        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                color = Color.White,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = subtitle,
                color = Color.White.copy(alpha = 0.6f),
                fontSize = 12.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun TrackRow(
    track: Track,
    selected: Boolean,
    playing: Boolean,
    accent: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val background by animateColorAsState(
        targetValue = if (selected) Color.White.copy(alpha = 0.12f) else Color.Transparent,
        animationSpec = tween(220),
        label = "fila-seleccionada",
    )

    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 62.dp)
            .clip(RoundedCornerShape(18.dp))
            .background(background)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .semantics {
                stateDescription = when {
                    playing -> "Reproduciendo"
                    selected -> "Seleccionada, en pausa"
                    else -> track.durationLabel
                }
            }
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier = Modifier
                .size(44.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(
                    Brush.linearGradient(
                        colors = listOf(
                            lerp(
                                accent,
                                Color.Black,
                                if (selected) 0.25f else 0.55f,
                            ),
                            Color(0xFF161626),
                        ),
                    ),
                ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = if (selected) Icons.Rounded.GraphicEq else Icons.Rounded.MusicNote,
                contentDescription = null,
                tint = if (selected) Color.White else Color.White.copy(alpha = 0.55f),
                modifier = Modifier.size(20.dp),
            )
        }

        Spacer(Modifier.width(12.dp))

        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = track.title,
                color = Color.White,
                fontSize = 14.sp,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = track.artist,
                color = Color.White.copy(alpha = 0.6f),
                fontSize = 12.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }

        Spacer(Modifier.width(8.dp))

        if (playing) {
            Equalizer(
                playing = true,
                color = accent,
                modifier = Modifier.size(width = 16.dp, height = 16.dp),
            )
        } else {
            Text(
                text = track.durationLabel,
                color = Color.White.copy(alpha = 0.55f),
                fontSize = 11.sp,
            )
        }
    }
}

/**
 * Tarjeta reutilizable para los estados de permiso, carga, biblioteca vacía y
 * error. Siempre ofrece una acción clara y, si aplica, una secundaria.
 */
@Composable
fun StatusCard(
    backdrop: Backdrop,
    icon: ImageVector,
    title: String,
    message: String,
    modifier: Modifier = Modifier,
    primaryLabel: String? = null,
    onPrimary: (() -> Unit)? = null,
    secondaryLabel: String? = null,
    onSecondary: (() -> Unit)? = null,
    showProgress: Boolean = false,
) {
    GlassPanel(
        backdrop = backdrop,
        modifier = modifier.fillMaxWidth(),
        contentPadding = PaddingValues(horizontal = 24.dp, vertical = 28.dp),
    ) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = Color.White.copy(alpha = 0.85f),
            modifier = Modifier
                .size(44.dp)
                .align(Alignment.CenterHorizontally),
        )
        Spacer(Modifier.height(14.dp))
        Text(
            text = title,
            color = Color.White,
            fontSize = 20.sp,
            fontWeight = FontWeight.Bold,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        Text(
            text = message,
            color = Color.White.copy(alpha = 0.68f),
            fontSize = 13.sp,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )

        if (showProgress) {
            Spacer(Modifier.height(20.dp))
            CircularProgressIndicator(
                color = Color.White,
                strokeWidth = 3.dp,
                modifier = Modifier
                    .size(28.dp)
                    .align(Alignment.CenterHorizontally),
            )
        }

        Spacer(Modifier.height(22.dp))
        if (primaryLabel != null && onPrimary != null) {
            Button(
                onClick = onPrimary,
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(min = 48.dp),
                shape = RoundedCornerShape(16.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = Color(0x33FFFFFF),
                    contentColor = Color.White,
                ),
                border = BorderStroke(1.dp, Color(0x59FFFFFF)),
            ) {
                Text(
                    text = primaryLabel,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }

        if (secondaryLabel != null && onSecondary != null) {
            TextButton(
                onClick = onSecondary,
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(min = 48.dp),
            ) {
                Text(
                    text = secondaryLabel,
                    color = Color.White.copy(alpha = 0.72f),
                    fontSize = 13.sp,
                )
            }
        }
    }
}
