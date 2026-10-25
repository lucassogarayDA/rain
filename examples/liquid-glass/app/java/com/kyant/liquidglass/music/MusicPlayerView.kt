package com.kyant.liquidglass.music

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.selection.selectable
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Pause
import androidx.compose.material.icons.rounded.PlayArrow
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material.icons.rounded.SkipNext
import androidx.compose.material.icons.rounded.SkipPrevious
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay

data class Track(
    val title: String,
    val artist: String,
    val durationSeconds: Int,
) {
    val durationLabel: String get() = formatTime(durationSeconds)
}

/** Formatea segundos como m:ss sin depender de la configuración regional. */
private fun formatTime(totalSeconds: Int): String {
    val safe = totalSeconds.coerceAtLeast(0)
    val minutes = safe / 60
    val seconds = safe % 60
    return "$minutes:${seconds.toString().padStart(2, '0')}"
}

val SamplePlaylist = listOf(
    Track("Liquid Glass Dreams", "Kyant & Backdrop", 222),
    Track("Aesthetic Horizons", "Glassmorphic Beats", 255),
    Track("Ambient Refraction", "Neon Crystal", 178),
    Track("Prism Wave", "Synthesizer Echo", 210),
    Track("Luminous Flow", "Android Compose Lab", 245),
)

@Composable
fun MusicPlayerView(
    autoplay: Boolean,
    onResetOnboarding: () -> Unit,
) {
    var currentTrackIndex by rememberSaveable { mutableIntStateOf(0) }
    var isPlaying by rememberSaveable { mutableStateOf(autoplay) }
    var trackProgress by rememberSaveable { mutableFloatStateOf(0f) }

    val track = SamplePlaylist[currentTrackIndex]

    val rotateAnim = remember { Animatable(0f) }
    LaunchedEffect(isPlaying) {
        if (isPlaying) {
            rotateAnim.animateTo(
                targetValue = 360f,
                animationSpec = infiniteRepeatable(
                    animation = tween(12000, easing = LinearEasing),
                    repeatMode = RepeatMode.Restart,
                ),
            )
        }
    }

    // Progreso real simulado: avanza con el reloj y salta a la siguiente pista
    // al llegar al final. La lista es de ejemplo; no hay audio que reproducir.
    LaunchedEffect(isPlaying, currentTrackIndex) {
        if (!isPlaying) return@LaunchedEffect
        while (true) {
            delay(250)
            val next = trackProgress + 0.25f / track.durationSeconds.toFloat()
            if (next >= 1f) {
                currentTrackIndex = (currentTrackIndex + 1) % SamplePlaylist.size
                trackProgress = 0f
            } else {
                trackProgress = next
            }
        }
    }

    LiquidGlassBackground {
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = 20.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "Liquid Glass Player",
                            color = Color.White,
                            fontSize = 22.sp,
                            fontWeight = FontWeight.Bold,
                        )
                        Text(
                            text = "Demo · ${SamplePlaylist.size} pistas de ejemplo",
                            color = Color.White.copy(alpha = 0.6f),
                            fontSize = 12.sp,
                        )
                    }
                    LiquidGlassIconButton(
                        icon = Icons.Rounded.Settings,
                        contentDescription = "Ajustes: volver a la introducción",
                        onClick = onResetOnboarding,
                    )
                }
            }

            item {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 8.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    LiquidGlassCard(
                        modifier = Modifier
                            .fillMaxWidth(0.62f)
                            .widthIn(max = 220.dp)
                            .aspectRatio(1f),
                        cornerRadius = 999.dp,
                    ) {
                        Canvas(modifier = Modifier.fillMaxSize()) {
                            rotate(rotateAnim.value) {
                                drawCircle(
                                    brush = Brush.sweepGradient(
                                        colors = listOf(
                                            Color(0xFF818CF8),
                                            Color(0xFFEC4899),
                                            Color(0xFF38BDF8),
                                            Color(0xFF818CF8),
                                        ),
                                    ),
                                    radius = size.width * 0.45f,
                                    center = center,
                                )
                                drawCircle(
                                    color = Color(0xFF0F172A),
                                    radius = size.width * 0.18f,
                                    center = center,
                                )
                            }
                        }
                    }
                }
            }

            item {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Text(
                        text = track.title,
                        color = Color.White,
                        fontSize = 22.sp,
                        fontWeight = FontWeight.Bold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        textAlign = TextAlign.Center,
                    )
                    Text(
                        text = track.artist,
                        color = Color.White.copy(alpha = 0.7f),
                        fontSize = 15.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        textAlign = TextAlign.Center,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                    Text(
                        text = "Pista ${currentTrackIndex + 1} de ${SamplePlaylist.size}",
                        color = Color.White.copy(alpha = 0.5f),
                        fontSize = 11.sp,
                        modifier = Modifier.padding(top = 6.dp),
                    )
                }
            }

            item {
                Column(modifier = Modifier.fillMaxWidth()) {
                    LiquidGlassSlider(
                        progress = trackProgress,
                        onProgressChange = { trackProgress = it },
                        contentDescription = "Posición de ${track.title}",
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(
                            text = formatTime((trackProgress * track.durationSeconds).toInt()),
                            color = Color.White.copy(alpha = 0.5f),
                            fontSize = 12.sp,
                        )
                        Text(
                            text = track.durationLabel,
                            color = Color.White.copy(alpha = 0.5f),
                            fontSize = 12.sp,
                        )
                    }
                }
            }

            item {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 8.dp),
                    horizontalArrangement = Arrangement.SpaceEvenly,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    LiquidGlassIconButton(
                        icon = Icons.Rounded.SkipPrevious,
                        contentDescription = "Pista anterior",
                        onClick = {
                            if (trackProgress > 0.03f) {
                                trackProgress = 0f
                            } else {
                                currentTrackIndex =
                                    if (currentTrackIndex > 0) currentTrackIndex - 1 else SamplePlaylist.size - 1
                                trackProgress = 0f
                            }
                        },
                    )

                    LiquidGlassIconButton(
                        icon = if (isPlaying) Icons.Rounded.Pause else Icons.Rounded.PlayArrow,
                        contentDescription = if (isPlaying) "Pausar" else "Reproducir",
                        onClick = { isPlaying = !isPlaying },
                    )

                    LiquidGlassIconButton(
                        icon = Icons.Rounded.SkipNext,
                        contentDescription = "Pista siguiente",
                        onClick = {
                            currentTrackIndex = (currentTrackIndex + 1) % SamplePlaylist.size
                            trackProgress = 0f
                        },
                    )
                }
            }

            item {
                Text(
                    text = "Lista de reproducción",
                    color = Color.White,
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(top = 8.dp, bottom = 4.dp),
                )
            }

            itemsIndexed(
                items = SamplePlaylist,
                key = { _, item -> item.title },
            ) { index, item ->
                val isSelected = index == currentTrackIndex
                LiquidGlassCard(
                    modifier = Modifier
                        .fillMaxWidth()
                        .selectable(
                            selected = isSelected,
                            role = Role.RadioButton,
                            onClick = {
                                currentTrackIndex = index
                                isPlaying = true
                                trackProgress = 0f
                            },
                        ),
                    cornerRadius = 14.dp,
                    tint = if (isSelected) Color(0x44818CF8) else Color.White.copy(alpha = 0.08f),
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(14.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = item.title,
                                color = if (isSelected) Color(0xFFF472B6) else Color.White,
                                fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            Text(
                                text = item.artist,
                                color = Color.White.copy(alpha = 0.6f),
                                fontSize = 12.sp,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            if (isSelected) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    modifier = Modifier.padding(top = 2.dp),
                                ) {
                                    Icon(
                                        imageVector = if (isPlaying) Icons.Rounded.PlayArrow else Icons.Rounded.Pause,
                                        contentDescription = null,
                                        tint = Color.White.copy(alpha = 0.8f),
                                        modifier = Modifier.size(14.dp),
                                    )
                                    Text(
                                        text = if (isPlaying) "Sonando ahora" else "En pausa",
                                        color = Color.White.copy(alpha = 0.8f),
                                        fontSize = 11.sp,
                                        modifier = Modifier.padding(start = 4.dp),
                                    )
                                }
                            }
                        }
                        Text(
                            text = item.durationLabel,
                            color = Color.White.copy(alpha = 0.5f),
                            fontSize = 13.sp,
                        )
                    }
                }
            }
        }
    }
}
