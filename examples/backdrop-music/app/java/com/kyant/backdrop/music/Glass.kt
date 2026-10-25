package com.kyant.backdrop.music

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode as RepeatAnimation
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Album
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.Home
import androidx.compose.material.icons.rounded.LibraryMusic
import androidx.compose.material.icons.rounded.Pause
import androidx.compose.material.icons.rounded.PlayArrow
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.setProgress
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kyant.backdrop.Backdrop
import com.kyant.backdrop.backdrops.LayerBackdrop
import com.kyant.backdrop.backdrops.layerBackdrop
import com.kyant.backdrop.backdrops.rememberCombinedBackdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop
import com.kyant.backdrop.drawBackdrop
import com.kyant.backdrop.effects.blur
import com.kyant.backdrop.effects.lens
import com.kyant.backdrop.effects.vibrancy
import com.kyant.shapes.Capsule
import kotlin.math.sin

/**
 * Fondo que se graba en el [LayerBackdrop]: degradado, carátula atenuada y dos
 * halos que toman el acento de la pista y se mueven lentamente. Todo lo que
 * dibuja esta función es lo que después muestran los paneles de cristal.
 */
@Composable
fun SceneBackground(
    artwork: ImageBitmap?,
    accent: Color,
    modifier: Modifier = Modifier,
    reduceMotion: Boolean = false,
) {
    val drift = if (reduceMotion) {
        0.5f
    } else {
        val transition = rememberInfiniteTransition(label = "escena")
        val animatedDrift by transition.animateFloat(
            initialValue = 0f,
            targetValue = 1f,
            animationSpec = infiniteRepeatable(
                animation = tween(durationMillis = 24_000, easing = LinearEasing),
                repeatMode = RepeatAnimation.Reverse,
            ),
            label = "deriva",
        )
        animatedDrift
    }
    val glow = lerp(accent, Color(0xFFEC4899), 0.45f)

    Box(modifier = modifier.fillMaxSize()) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(
                    Brush.verticalGradient(
                        colors = listOf(
                            lerp(accent, Color(0xFF07070C), 0.86f),
                            Color(0xFF131024),
                            Color(0xFF05050A),
                        ),
                    ),
                ),
        )

        // La carátula se funde al cambiar de pista; el fondo no salta.
        Crossfade(
            targetState = artwork,
            modifier = Modifier.fillMaxSize(),
            animationSpec = tween(durationMillis = if (reduceMotion) 0 else 700),
            label = "fondo-carátula",
        ) { image ->
            if (image != null) {
                Image(
                    bitmap = image,
                    contentDescription = null,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Color.Black.copy(alpha = 0.1f)),
                )
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(
                            Brush.verticalGradient(
                                colors = listOf(
                                    Color(0xB305050A),
                                    Color(0x3305050A),
                                    Color(0xF205050A),
                                ),
                            ),
                        ),
                )
            }
        }

        Canvas(modifier = Modifier.fillMaxSize()) {
            val firstCenter = Offset(
                x = size.width * (0.84f - 0.10f * drift),
                y = size.height * (0.14f + 0.06f * drift),
            )
            drawCircle(
                brush = Brush.radialGradient(
                    colors = listOf(accent.copy(alpha = 0.55f), Color.Transparent),
                    center = firstCenter,
                    radius = size.minDimension * 0.8f,
                ),
                radius = size.minDimension * 0.8f,
                center = firstCenter,
            )

            val secondCenter = Offset(
                x = size.width * (0.12f + 0.10f * drift),
                y = size.height * (0.82f - 0.08f * drift),
            )
            drawCircle(
                brush = Brush.radialGradient(
                    colors = listOf(glow.copy(alpha = 0.42f), Color.Transparent),
                    center = secondCenter,
                    radius = size.minDimension * 0.9f,
                ),
                radius = size.minDimension * 0.9f,
                center = secondCenter,
            )
        }
    }
}

/**
 * Panel de cristal real: el fondo grabado se satura con [vibrancy], se
 * desenfoca con [blur] y se refracta en los bordes con [lens]. El orden de los
 * efectos es el que exige la documentación: color → desenfoque → lente.
 *
 * [exportedBackdrop] permite que otro cristal hijo (por ejemplo el buscador de
 * la biblioteca) refracte este panel en vez del fondo, sin crear un bucle de
 * grabación.
 */
@Composable
fun GlassPanel(
    backdrop: Backdrop,
    modifier: Modifier = Modifier,
    shape: androidx.compose.ui.graphics.Shape = RoundedCornerShape(28.dp),
    surfaceTint: Color = Color.White.copy(alpha = 0.10f),
    contentPadding: PaddingValues = PaddingValues(20.dp),
    exportedBackdrop: LayerBackdrop? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier = modifier
            .drawBackdrop(
                backdrop = backdrop,
                shape = { shape },
                effects = {
                    vibrancy()
                    blur(18.dp.toPx())
                    lens(
                        refractionHeight = 16.dp.toPx(),
                        refractionAmount = 24.dp.toPx(),
                        depthEffect = true,
                        chromaticAberration = true,
                    )
                },
                exportedBackdrop = exportedBackdrop,
                onDrawSurface = { drawRect(surfaceTint) },
            )
            .padding(contentPadding),
        content = content,
    )
}

/**
 * Botón circular de cristal con pulsación animada. La escala va en [layerBlock]
 * para que el contenido se encoja pero el fondo refractado no se deforme, tal
 * como explica el tutorial de la barra inferior.
 */
@Composable
fun GlassIconButton(
    backdrop: Backdrop,
    icon: ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    active: Boolean = false,
    accent: Color = Color.White,
    buttonSize: Dp = 52.dp,
    iconSize: Dp = 24.dp,
) {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed && enabled) 0.92f else 1f,
        animationSpec = spring(stiffness = Spring.StiffnessMediumLow),
        label = "pulsación-botón",
    )
    val tint = when {
        !enabled -> Color.White.copy(alpha = 0.35f)
        active -> Color.White
        else -> Color.White.copy(alpha = 0.86f)
    }
    val surface = when {
        active -> accent.copy(alpha = 0.62f)
        pressed -> Color.White.copy(alpha = 0.24f)
        else -> Color.White.copy(alpha = 0.13f)
    }

    Box(
        modifier = modifier
            .size(buttonSize)
            .drawBackdrop(
                backdrop = backdrop,
                shape = { CircleShape },
                effects = {
                    vibrancy()
                    blur(8.dp.toPx())
                    lens(
                        refractionHeight = 10.dp.toPx(),
                        refractionAmount = 14.dp.toPx(),
                        depthEffect = true,
                    )
                },
                shadow = null,
                layerBlock = {
                    scaleX = scale
                    scaleY = scale
                },
                onDrawSurface = { drawCircle(color = surface) },
            )
            .clickable(
                interactionSource = interaction,
                indication = null,
                enabled = enabled,
                role = Role.Button,
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = icon,
            contentDescription = contentDescription,
            tint = tint,
            modifier = Modifier.size(iconSize),
        )
    }
}

/** Botón principal de reproducción, más grande y teñido con el acento. */
@Composable
fun GlassPlayButton(
    backdrop: Backdrop,
    isPlaying: Boolean,
    isPreparing: Boolean,
    accent: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    buttonSize: Dp = 74.dp,
) {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed && !isPreparing) 0.9f else 1f,
        animationSpec = spring(stiffness = Spring.StiffnessMediumLow),
        label = "pulsación-reproducir",
    )

    Box(
        modifier = modifier
            .size(buttonSize)
            .drawBackdrop(
                backdrop = backdrop,
                shape = { CircleShape },
                effects = {
                    vibrancy()
                    blur(12.dp.toPx())
                    lens(
                        refractionHeight = 12.dp.toPx(),
                        refractionAmount = 18.dp.toPx(),
                        depthEffect = true,
                        chromaticAberration = true,
                    )
                },
                shadow = null,
                layerBlock = {
                    scaleX = scale
                    scaleY = scale
                },
                onDrawSurface = {
                    drawCircle(color = accent.copy(alpha = 0.78f))
                    drawCircle(color = Color.White.copy(alpha = 0.10f))
                },
            )
            .clickable(
                interactionSource = interaction,
                indication = null,
                enabled = !isPreparing,
                role = Role.Button,
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        if (isPreparing) {
            CircularProgressIndicator(
                color = Color.White,
                strokeWidth = 3.dp,
                modifier = Modifier.size(28.dp),
            )
        } else {
            Icon(
                imageVector = if (isPlaying) {
                    androidx.compose.material.icons.Icons.Rounded.Pause
                } else {
                    androidx.compose.material.icons.Icons.Rounded.PlayArrow
                },
                contentDescription = if (isPlaying) "Pausar" else "Reproducir",
                tint = Color.White,
                modifier = Modifier.size(36.dp),
            )
        }
    }
}

/**
 * Barra de progreso de cristal. La pista se graba en su propio [LayerBackdrop]
 * y el pulgar combina fondo y pista con [rememberCombinedBackdrop], así que
 * refracta ambos a la vez.
 */
@Composable
fun GlassSeekBar(
    backdrop: Backdrop,
    positionMs: Long,
    durationMs: Long,
    accent: Color,
    onSeek: (Long) -> Unit,
    modifier: Modifier = Modifier,
) {
    val duration = durationMs.coerceAtLeast(1L)
    var dragFraction by remember { mutableStateOf<Float?>(null) }
    val playedFraction = (positionMs.toFloat() / duration).coerceIn(0f, 1f)
    val fraction = dragFraction ?: playedFraction
    val trackBackdrop = rememberLayerBackdrop()
    val combined = rememberCombinedBackdrop(backdrop, trackBackdrop)

    fun seekToFraction(value: Float) {
        onSeek((value.coerceIn(0f, 1f) * duration).toLong())
    }

    Column(
        modifier = modifier
            .fillMaxWidth()
            .semantics {
                contentDescription = "Posición de reproducción"
                progressBarRangeInfo = ProgressBarRangeInfo(fraction, 0f..1f)
                stateDescription =
                    "${formatDuration((fraction * duration).toLong())} de ${formatDuration(duration)}"
                setProgress { target ->
                    seekToFraction(target)
                    true
                }
            },
    ) {
        BoxWithConstraints(
            modifier = Modifier
                .fillMaxWidth()
                .height(44.dp)
                .pointerInput(duration) {
                    detectTapGestures { offset ->
                        if (size.width > 0) seekToFraction(offset.x / size.width)
                    }
                }
                .pointerInput(duration) {
                    detectHorizontalDragGestures(
                        onDragStart = { offset ->
                            if (size.width > 0) {
                                dragFraction = (offset.x / size.width).coerceIn(0f, 1f)
                            }
                        },
                        onHorizontalDrag = { change, _ ->
                            change.consume()
                            if (size.width > 0) {
                                dragFraction = (change.position.x / size.width).coerceIn(0f, 1f)
                            }
                        },
                        onDragEnd = {
                            val committed = dragFraction
                            dragFraction = null
                            if (committed != null) seekToFraction(committed)
                        },
                        onDragCancel = { dragFraction = null },
                    )
                },
        ) {
            val trackHeight = 8.dp
            val thumbWidth = 44.dp
            val thumbHeight = 26.dp

            Box(
                modifier = Modifier
                    .layerBackdrop(trackBackdrop)
                    .align(Alignment.CenterStart)
                    .fillMaxWidth()
                    .height(trackHeight)
                    .clip(Capsule())
                    .background(Color.White.copy(alpha = 0.18f)),
            ) {
                if (fraction > 0f) {
                    Box(
                        modifier = Modifier
                            .fillMaxHeight()
                            .fillMaxWidth(fraction)
                            .background(
                                Brush.horizontalGradient(
                                    colors = listOf(
                                        Color.White.copy(alpha = 0.92f),
                                        accent,
                                    ),
                                ),
                            ),
                    )
                }
            }

            Box(
                modifier = Modifier
                    .align(Alignment.CenterStart)
                    .offset(x = (maxWidth - thumbWidth) * fraction)
                    .size(thumbWidth, thumbHeight)
                    .drawBackdrop(
                        backdrop = combined,
                        shape = { Capsule() },
                        effects = {
                            vibrancy()
                            blur(4.dp.toPx())
                            lens(
                                refractionHeight = 10.dp.toPx(),
                                refractionAmount = 16.dp.toPx(),
                                depthEffect = true,
                                chromaticAberration = true,
                            )
                        },
                        shadow = null,
                        onDrawSurface = { drawRect(Color.White.copy(alpha = 0.32f)) },
                    ),
            )
        }

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(
                text = formatDuration((fraction * duration).toLong()),
                color = Color.White.copy(alpha = 0.68f),
                fontSize = 12.sp,
            )
            Text(
                text = formatDuration(duration),
                color = Color.White.copy(alpha = 0.68f),
                fontSize = 12.sp,
            )
        }
    }
}

/**
 * Carátula de la pista con fundido y zoom al cambiar de canción. Si el archivo
 * no trae imagen se muestra un degradado con el acento dominante.
 */
@Composable
fun ArtworkCard(
    artwork: ImageBitmap?,
    accent: Color,
    isPlaying: Boolean,
    isPreparing: Boolean,
    trackTitle: String,
    modifier: Modifier = Modifier,
) {
    val shape = RoundedCornerShape(32.dp)
    val status = when {
        isPreparing -> "Cargando"
        isPlaying -> "Reproduciendo"
        else -> "En pausa"
    }

    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(300.dp)
            .clip(shape)
            .background(
                brush = Brush.linearGradient(
                    colors = listOf(
                        lerp(accent, Color.Black, 0.42f),
                        Color(0xFF0C0C16),
                    ),
                ),
            )
            .border(
                width = 1.dp,
                brush = Brush.verticalGradient(
                    colors = listOf(
                        Color.White.copy(alpha = 0.35f),
                        Color.White.copy(alpha = 0.06f),
                    ),
                ),
                shape = shape,
            ),
        contentAlignment = Alignment.Center,
    ) {
        AnimatedContent(
            targetState = artwork,
            transitionSpec = {
                (fadeIn(tween(520)) + scaleIn(initialScale = 0.92f, animationSpec = tween(520)))
                    .togetherWith(fadeOut(tween(260)))
            },
            modifier = Modifier.fillMaxSize(),
            label = "carátula",
        ) { image ->
            if (image != null) {
                Image(
                    bitmap = image,
                    contentDescription = "Carátula de $trackTitle",
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
            } else {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(
                        imageVector = Icons.Rounded.Album,
                        contentDescription = null,
                        tint = Color.White.copy(alpha = 0.35f),
                        modifier = Modifier.size(88.dp),
                    )
                }
            }
        }

        Row(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .padding(14.dp)
                .background(Color(0x8C000000), Capsule())
                .padding(horizontal = 12.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (isPlaying) {
                Equalizer(
                    playing = true,
                    color = accent,
                    modifier = Modifier.size(width = 16.dp, height = 14.dp),
                )
                Spacer(Modifier.width(8.dp))
            }
            Text(
                text = status,
                color = Color.White,
                fontSize = 12.sp,
                fontWeight = FontWeight.Medium,
            )
        }
    }
}

/** Cuatro barras animadas que se mueven solo mientras hay reproducción. */
@Composable
fun Equalizer(
    playing: Boolean,
    color: Color,
    modifier: Modifier = Modifier,
    barCount: Int = 4,
) {
    val transition = rememberInfiniteTransition(label = "ecualizador")
    val phase by transition.animateFloat(
        initialValue = 0f,
        targetValue = (2f * Math.PI).toFloat(),
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1500, easing = LinearEasing),
        ),
        label = "fase",
    )
    val level by animateFloatAsState(
        targetValue = if (playing) 1f else 0f,
        animationSpec = tween(durationMillis = 350),
        label = "nivel",
    )

    Canvas(modifier = modifier) {
        if (barCount <= 0 || size.width <= 0f) return@Canvas
        val slot = size.width / barCount
        val barWidth = slot * 0.52f
        for (index in 0 until barCount) {
            val wave = (sin(phase + index * 1.15f) + 1f) / 2f
            val heightFraction = 0.18f + 0.82f * wave * level
            val barHeight = size.height * heightFraction
            drawRoundRect(
                color = color,
                topLeft = Offset(
                    x = index * slot + (slot - barWidth) / 2f,
                    y = size.height - barHeight,
                ),
                size = Size(barWidth, barHeight),
                cornerRadius = CornerRadius(barWidth / 2f),
            )
        }
    }
}

/**
 * Mini-reproductor flotante que aparece sobre la navegación mientras se
 * consulta la biblioteca. La línea inferior refleja el progreso real.
 */
@Composable
fun MiniPlayer(
    backdrop: Backdrop,
    track: Track,
    artwork: ImageBitmap?,
    isPlaying: Boolean,
    isPreparing: Boolean,
    progress: Float,
    accent: Color,
    onClick: () -> Unit,
    onToggle: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .height(64.dp)
            .drawBackdrop(
                backdrop = backdrop,
                shape = { RoundedCornerShape(22.dp) },
                effects = {
                    vibrancy()
                    blur(16.dp.toPx())
                    lens(
                        refractionHeight = 12.dp.toPx(),
                        refractionAmount = 18.dp.toPx(),
                        depthEffect = true,
                    )
                },
                onDrawSurface = { drawRect(Color.White.copy(alpha = 0.09f)) },
                onDrawFront = {
                    if (progress > 0f) {
                        drawRect(
                            color = accent.copy(alpha = 0.95f),
                            topLeft = Offset(0f, size.height - 2.dp.toPx()),
                            size = Size(size.width * progress, 2.dp.toPx()),
                        )
                    }
                },
            )
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier = Modifier
                .size(44.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(
                    Brush.linearGradient(
                        colors = listOf(
                            lerp(accent, Color.Black, 0.3f),
                            Color(0xFF141422),
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
                    tint = Color.White.copy(alpha = 0.6f),
                    modifier = Modifier.size(22.dp),
                )
            }
        }

        Spacer(Modifier.width(12.dp))

        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = track.title,
                color = Color.White,
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = track.artist,
                color = Color.White.copy(alpha = 0.62f),
                fontSize = 12.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }

        Equalizer(
            playing = isPlaying,
            color = accent,
            modifier = Modifier.size(width = 16.dp, height = 16.dp),
        )
        Spacer(Modifier.width(10.dp))

        Box(
            modifier = Modifier
                .size(42.dp)
                .clip(CircleShape)
                .clickable(
                    interactionSource = null,
                    indication = null,
                    role = Role.Button,
                    onClick = onToggle,
                ),
            contentAlignment = Alignment.Center,
        ) {
            if (isPreparing) {
                CircularProgressIndicator(
                    color = Color.White,
                    strokeWidth = 2.dp,
                    modifier = Modifier.size(20.dp),
                )
            } else {
                Icon(
                    imageVector = if (isPlaying) {
                        androidx.compose.material.icons.Icons.Rounded.Pause
                    } else {
                        androidx.compose.material.icons.Icons.Rounded.PlayArrow
                    },
                    contentDescription = if (isPlaying) "Pausar" else "Reproducir",
                    tint = Color.White,
                    modifier = Modifier.size(24.dp),
                )
            }
        }
    }
}

/**
 * Buscador de cristal. Dentro de un panel se le pasa el backdrop exportado por
 * el panel y [nested] reduce el desenfoque para no engordar la lente.
 */
@Composable
fun GlassSearchField(
    backdrop: Backdrop,
    query: String,
    onQueryChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    accent: Color = Color.White,
    nested: Boolean = false,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .height(52.dp)
            .drawBackdrop(
                backdrop = backdrop,
                shape = { Capsule() },
                effects = {
                    vibrancy()
                    blur((if (nested) 6f else 14f).dp.toPx())
                    lens(
                        refractionHeight = 10.dp.toPx(),
                        refractionAmount = 16.dp.toPx(),
                    )
                },
                shadow = null,
                onDrawSurface = { drawRect(Color.White.copy(alpha = 0.10f)) },
            )
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = Icons.Rounded.Search,
            contentDescription = null,
            tint = Color.White.copy(alpha = 0.6f),
            modifier = Modifier.size(20.dp),
        )
        Spacer(Modifier.width(10.dp))

        Box(modifier = Modifier.weight(1f)) {
            if (query.isEmpty()) {
                Text(
                    text = "Buscar por título, artista o álbum",
                    color = Color.White.copy(alpha = 0.5f),
                    fontSize = 14.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            BasicTextField(
                value = query,
                onValueChange = onQueryChange,
                singleLine = true,
                textStyle = TextStyle(color = Color.White, fontSize = 14.sp),
                cursorBrush = SolidColor(accent),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                modifier = Modifier.fillMaxWidth(),
            )
        }

        if (query.isNotEmpty()) {
            Box(
                modifier = Modifier
                    .size(34.dp)
                    .clip(CircleShape)
                    .clickable(role = Role.Button, onClick = { onQueryChange("") }),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    imageVector = Icons.Rounded.Close,
                    contentDescription = "Borrar búsqueda",
                    tint = Color.White.copy(alpha = 0.75f),
                    modifier = Modifier.size(18.dp),
                )
            }
        }
    }
}

/**
 * Navegación inferior de Material 3 sobre cristal: conserva la semántica y el
 * indicador de selección de [NavigationBar], pero el fondo se refracta.
 */
@Composable
fun GlassNavigationBar(
    backdrop: Backdrop,
    selected: Int,
    onSelect: (Int) -> Unit,
    accent: Color,
    modifier: Modifier = Modifier,
) {
    NavigationBar(
        modifier = modifier
            .fillMaxWidth()
            .drawBackdrop(
                backdrop = backdrop,
                shape = { RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp) },
                effects = {
                    vibrancy()
                    blur(20.dp.toPx())
                    lens(
                        refractionHeight = 14.dp.toPx(),
                        refractionAmount = 20.dp.toPx(),
                    )
                },
                onDrawSurface = { drawRect(Color.White.copy(alpha = 0.07f)) },
            ),
        containerColor = Color.Transparent,
        tonalElevation = 0.dp,
        windowInsets = WindowInsets(0, 0, 0, 0),
    ) {
        NavigationBarItem(
            selected = selected == 0,
            onClick = { onSelect(0) },
            icon = {
                Icon(
                    imageVector = Icons.Rounded.Home,
                    contentDescription = "Inicio",
                )
            },
            label = { Text("Inicio", fontSize = 11.sp) },
            colors = navigationColors(accent),
        )
        NavigationBarItem(
            selected = selected == 1,
            onClick = { onSelect(1) },
            icon = {
                Icon(
                    imageVector = Icons.Rounded.LibraryMusic,
                    contentDescription = "Biblioteca",
                )
            },
            label = { Text("Biblioteca", fontSize = 11.sp) },
            colors = navigationColors(accent),
        )
        NavigationBarItem(
            selected = selected == 2,
            onClick = { onSelect(2) },
            icon = {
                Icon(
                    imageVector = Icons.Rounded.Album,
                    contentDescription = "Reproductor",
                )
            },
            label = { Text("Reproductor", fontSize = 11.sp) },
            colors = navigationColors(accent),
        )
        NavigationBarItem(
            selected = selected == 3,
            onClick = { onSelect(3) },
            icon = {
                Icon(
                    imageVector = Icons.Rounded.Settings,
                    contentDescription = "Configuración",
                )
            },
            label = { Text("Ajustes", fontSize = 11.sp) },
            colors = navigationColors(accent),
        )
    }
}

@Composable
private fun navigationColors(accent: Color) = NavigationBarItemDefaults.colors(
    selectedIconColor = Color.White,
    selectedTextColor = Color.White,
    indicatorColor = accent.copy(alpha = 0.32f),
    unselectedIconColor = Color.White.copy(alpha = 0.55f),
    unselectedTextColor = Color.White.copy(alpha = 0.55f),
)

/** Etiqueta que explica qué cristal puede ofrecer cada versión de Android. */
fun glassSupportLabel(): String = when {
    android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.TIRAMISU ->
        "Cristal con desenfoque y refracción reales."
    android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.S ->
        "Desenfoque real; la refracción necesita Android 13."
    else ->
        "Sin desenfoque en este Android: tinte translúcido."
}
