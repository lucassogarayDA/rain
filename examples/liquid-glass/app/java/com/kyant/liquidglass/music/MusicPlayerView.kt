package com.kyant.liquidglass.music

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

data class Track(
    val title: String,
    val artist: String,
    val duration: String,
)

val SamplePlaylist = listOf(
    Track("Liquid Glass Dreams", "Kyant & Backdrop", "3:42"),
    Track("Aesthetic Horizons", "Glassmorphic Beats", "4:15"),
    Track("Ambient Refraction", "Neon Crystal", "2:58"),
    Track("Prism Wave", "Synthesizer Echo", "3:30"),
    Track("Luminous Flow", "Android Compose Lab", "4:05"),
)

@Composable
fun MusicPlayerView(
    onResetOnboarding: () -> Unit,
) {
    var currentTrackIndex by remember { mutableIntStateOf(0) }
    var isPlaying by remember { mutableStateOf(true) }
    var trackProgress by remember { mutableFloatStateOf(0.35f) }

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

    LiquidGlassBackground {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 16.dp, bottom = 12.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = "Liquid Glass Player",
                    color = Color.White,
                    fontSize = 20.sp,
                    fontWeight = FontWeight.Bold,
                )
                Text(
                    text = "⚙️",
                    fontSize = 18.sp,
                    modifier = Modifier.clickable { onResetOnboarding() },
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            LiquidGlassCard(
                modifier = Modifier
                    .size(220.dp)
                    .clip(CircleShape),
                cornerRadius = 110.dp,
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

            Spacer(modifier = Modifier.height(20.dp))

            Text(
                text = track.title,
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = track.artist,
                color = Color.White.copy(alpha = 0.7f),
                fontSize = 15.sp,
                modifier = Modifier.padding(top = 4.dp),
            )

            Spacer(modifier = Modifier.height(20.dp))

            LiquidGlassSlider(
                progress = trackProgress,
                onProgressChange = { trackProgress = it },
                modifier = Modifier.fillMaxWidth(),
            )

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                Text("1:18", color = Color.White.copy(alpha = 0.5f), fontSize = 12.sp)
                Text(track.duration, color = Color.White.copy(alpha = 0.5f), fontSize = 12.sp)
            }

            Spacer(modifier = Modifier.height(16.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                LiquidGlassButton(
                    text = "⏮️",
                    onClick = {
                        currentTrackIndex = if (currentTrackIndex > 0) currentTrackIndex - 1 else SamplePlaylist.size - 1
                        trackProgress = 0f
                    },
                )

                LiquidGlassButton(
                    text = if (isPlaying) "⏸️ Pausa" else "▶️ Reproducir",
                    onClick = { isPlaying = !isPlaying },
                )

                LiquidGlassButton(
                    text = "⏭️",
                    onClick = {
                        currentTrackIndex = (currentTrackIndex + 1) % SamplePlaylist.size
                        trackProgress = 0f
                    },
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            Text(
                text = "Lista de Reproducción",
                color = Color.White,
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier
                    .align(Alignment.Start)
                    .padding(bottom = 8.dp),
            )

            LazyColumn(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                itemsIndexed(SamplePlaylist) { index, item ->
                    val isSelected = index == currentTrackIndex
                    LiquidGlassCard(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable {
                                currentTrackIndex = index
                                isPlaying = true
                                trackProgress = 0f
                            },
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
                                )
                                Text(
                                    text = item.artist,
                                    color = Color.White.copy(alpha = 0.6f),
                                    fontSize = 12.sp,
                                )
                            }
                            Text(
                                text = item.duration,
                                color = Color.White.copy(alpha = 0.5f),
                                fontSize = 13.sp,
                            )
                        }
                    }
                }
            }
        }
    }
}
