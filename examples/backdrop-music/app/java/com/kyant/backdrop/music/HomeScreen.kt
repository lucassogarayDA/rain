package com.kyant.backdrop.music

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Album
import androidx.compose.material.icons.rounded.LibraryMusic
import androidx.compose.material.icons.rounded.Shuffle
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kyant.backdrop.Backdrop

@Composable
fun HomeScreen(
    controller: PlayerController,
    backdrop: Backdrop,
    accent: Color,
    onOpenPlayer: () -> Unit,
    onOpenLibrary: () -> Unit,
    onOpenAlbum: (String) -> Unit,
    onShufflePlay: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val queue = controller.queue
    val recent = remember(queue) { queue.sortedByDescending { it.dateAdded }.take(10) }
    val albums = remember(queue) { TrackGroups.albums(queue).take(12) }
    val artists = remember(queue) { TrackGroups.artists(queue) }
    val current = controller.currentTrack
    val currentArtwork by rememberArtwork(context, current)

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(top = 6.dp, bottom = 20.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp),
    ) {
        item(key = "hero") {
            Box(modifier = Modifier.padding(horizontal = 16.dp)) {
                GlassPanel(
                    backdrop = backdrop,
                    modifier = Modifier.fillMaxWidth(),
                    contentPadding = PaddingValues(18.dp),
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        ArtworkThumb(
                            artwork = currentArtwork?.image,
                            accent = accent,
                            size = 82.dp,
                        )
                        Spacer(Modifier.width(14.dp))
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = if (controller.isPlaying) "SONANDO AHORA" else "CONTINÚA ESCUCHANDO",
                                color = accent,
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold,
                            )
                            Spacer(Modifier.height(4.dp))
                            Text(
                                text = current?.title ?: "Tu biblioteca está lista",
                                color = Color.White,
                                fontSize = 18.sp,
                                fontWeight = FontWeight.Bold,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis,
                            )
                            Text(
                                text = current?.artist ?: "Toca una canción para empezar",
                                color = Color.White.copy(alpha = 0.62f),
                                fontSize = 12.sp,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                        GlassPlayButton(
                            backdrop = backdrop,
                            isPlaying = controller.isPlaying,
                            isPreparing = controller.isPreparing,
                            accent = accent,
                            onClick = controller::toggle,
                            buttonSize = 58.dp,
                        )
                    }

                    Spacer(Modifier.height(14.dp))

                    LinearProgressIndicator(
                        progress = { controller.progress },
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(4.dp)
                            .clip(CircleShape),
                        color = accent,
                        trackColor = Color.White.copy(alpha = 0.16f),
                    )
                    Spacer(Modifier.height(6.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(
                            text = formatDuration(controller.positionMs),
                            color = Color.White.copy(alpha = 0.55f),
                            fontSize = 11.sp,
                        )
                        Text(
                            text = formatDuration(controller.durationMs),
                            color = Color.White.copy(alpha = 0.55f),
                            fontSize = 11.sp,
                        )
                    }

                    Spacer(Modifier.height(8.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        Button(
                            onClick = onOpenPlayer,
                            modifier = Modifier
                                .weight(1f)
                                .height(46.dp),
                            shape = RoundedCornerShape(16.dp),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = Color.White.copy(alpha = 0.16f),
                                contentColor = Color.White,
                            ),
                        ) {
                            Text("Abrir reproductor", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        }
                        Button(
                            onClick = onShufflePlay,
                            modifier = Modifier.height(46.dp),
                            shape = RoundedCornerShape(16.dp),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = accent.copy(alpha = 0.55f),
                                contentColor = Color.White,
                            ),
                        ) {
                            Icon(
                                imageVector = Icons.Rounded.Shuffle,
                                contentDescription = null,
                                modifier = Modifier.size(17.dp),
                            )
                            Spacer(Modifier.width(6.dp))
                            Text("Aleatorio", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        }
                    }
                }
            }
        }

        item(key = "recientes-titulo") {
            SectionHeader(
                title = "Recientes",
                subtitle = "Añadidas recientemente al dispositivo",
                actionLabel = "Ver biblioteca",
                onAction = onOpenLibrary,
            )
        }

        item(key = "recientes") {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(recent, key = { it.id }) { track ->
                    RecentCard(
                        track = track,
                        accent = accent,
                        onClick = {
                            controller.play(track)
                            onOpenPlayer()
                        },
                    )
                }
            }
        }

        item(key = "albumes-titulo") {
            SectionHeader(
                title = "Álbumes",
                subtitle = if (albums.size == 1) "1 álbum" else "${albums.size} álbumes",
                actionLabel = "Ver todos",
                onAction = onOpenLibrary,
            )
        }

        item(key = "albumes") {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(albums, key = { it.key }) { album ->
                    AlbumCard(
                        album = album,
                        accent = accent,
                        onClick = { onOpenAlbum(album.name) },
                    )
                }
            }
        }

        item(key = "resumen") {
            Box(modifier = Modifier.padding(horizontal = 16.dp)) {
                GlassPanel(
                    backdrop = backdrop,
                    modifier = Modifier.fillMaxWidth(),
                    contentPadding = PaddingValues(18.dp),
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(
                            imageVector = Icons.Rounded.LibraryMusic,
                            contentDescription = null,
                            tint = accent,
                            modifier = Modifier.size(22.dp),
                        )
                        Spacer(Modifier.width(10.dp))
                        Text(
                            text = "Tu biblioteca",
                            color = Color.White,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                    Spacer(Modifier.height(10.dp))
                    Row(modifier = Modifier.fillMaxWidth()) {
                        LibraryStat(
                            value = queue.size.toString(),
                            label = "pistas",
                            modifier = Modifier.weight(1f),
                        )
                        LibraryStat(
                            value = albums.size.toString(),
                            label = "álbumes",
                            modifier = Modifier.weight(1f),
                        )
                        LibraryStat(
                            value = artists.size.toString(),
                            label = "artistas",
                            modifier = Modifier.weight(1f),
                        )
                    }
                    Spacer(Modifier.height(12.dp))
                    TextButton(
                        onClick = onOpenLibrary,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(
                            text = "Explorar canciones, artistas y carpetas",
                            color = Color.White.copy(alpha = 0.78f),
                            fontSize = 13.sp,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun LibraryStat(value: String, label: String, modifier: Modifier = Modifier) {
    Column(modifier = modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            color = Color.White,
            fontSize = 22.sp,
            fontWeight = FontWeight.Bold,
        )
        Text(
            text = label,
            color = Color.White.copy(alpha = 0.55f),
            fontSize = 11.sp,
        )
    }
}

@Composable
fun SectionHeader(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                color = Color.White,
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
            )
            if (subtitle != null) {
                Text(
                    text = subtitle,
                    color = Color.White.copy(alpha = 0.55f),
                    fontSize = 11.sp,
                )
            }
        }
        if (actionLabel != null && onAction != null) {
            TextButton(onClick = onAction) {
                Text(
                    text = actionLabel,
                    color = Color.White.copy(alpha = 0.72f),
                    fontSize = 12.sp,
                )
            }
        }
    }
}

@Composable
private fun RecentCard(
    track: Track,
    accent: Color,
    onClick: () -> Unit,
) {
    val context = LocalContext.current
    val artwork by rememberArtwork(context, track)

    Column(
        modifier = Modifier
            .width(132.dp)
            .clip(RoundedCornerShape(20.dp))
            .clickable(role = Role.Button, onClick = onClick)
            .padding(4.dp),
    ) {
        ArtworkThumb(
            artwork = artwork?.image,
            accent = accent,
            size = 124.dp,
        )
        Spacer(Modifier.height(7.dp))
        Text(
            text = track.title,
            color = Color.White,
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(horizontal = 2.dp),
        )
        Text(
            text = track.artist,
            color = Color.White.copy(alpha = 0.58f),
            fontSize = 11.sp,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(horizontal = 2.dp),
        )
    }
}

@Composable
private fun AlbumCard(
    album: AlbumGroup,
    accent: Color,
    onClick: () -> Unit,
) {
    val context = LocalContext.current
    val artwork by rememberAlbumArtwork(context, album)

    Column(
        modifier = Modifier
            .width(148.dp)
            .clip(RoundedCornerShape(22.dp))
            .clickable(role = Role.Button, onClick = onClick)
            .padding(4.dp),
    ) {
        ArtworkThumb(
            artwork = artwork?.image,
            accent = accent,
            size = 140.dp,
            corner = 20.dp,
        )
        Spacer(Modifier.height(7.dp))
        Text(
            text = album.name,
            color = Color.White,
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(horizontal = 2.dp),
        )
        Text(
            text = album.artist,
            color = Color.White.copy(alpha = 0.58f),
            fontSize = 11.sp,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(horizontal = 2.dp),
        )
    }
}

/** Miniatura cuadrada de carátula con degradado de reserva. */
@Composable
fun ArtworkThumb(
    artwork: ImageBitmap?,
    accent: Color,
    size: androidx.compose.ui.unit.Dp,
    modifier: Modifier = Modifier,
    corner: androidx.compose.ui.unit.Dp = 18.dp,
) {
    Box(
        modifier = modifier
            .size(size)
            .clip(RoundedCornerShape(corner))
            .background(
                Brush.linearGradient(
                    colors = listOf(
                        lerp(accent, Color.Black, 0.35f),
                        Color(0xFF151526),
                    ),
                ),
            ),
        contentAlignment = Alignment.Center,
    ) {
        if (artwork != null) {
            Image(
                bitmap = artwork,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
        } else {
            Icon(
                imageVector = Icons.Rounded.Album,
                contentDescription = null,
                tint = Color.White.copy(alpha = 0.45f),
                modifier = Modifier.size(size / 2.6f),
            )
        }
    }
}
