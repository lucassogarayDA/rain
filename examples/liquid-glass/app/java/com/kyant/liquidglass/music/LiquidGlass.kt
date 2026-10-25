package com.kyant.liquidglass.music

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.setProgress
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun LiquidGlassBackground(
    modifier: Modifier = Modifier,
    content: @Composable BoxScope.() -> Unit,
) {
    val transition = remember { Animatable(0f) }
    LaunchedEffect(Unit) {
        transition.animateTo(
            targetValue = 1f,
            animationSpec = infiniteRepeatable(
                animation = tween(8000, easing = FastOutSlowInEasing),
                repeatMode = RepeatMode.Reverse,
            ),
        )
    }

    val phase = transition.value
    val gradient = Brush.radialGradient(
        colors = listOf(
            Color(0xFF0F172A),
            Color(0xFF1E1B4B),
            Color(0xFF311042),
            Color(0xFF09090B),
        ),
        center = Offset(200f + 300f * phase, 300f + 400f * phase),
        radius = 1200f,
    )

    Box(
        modifier = modifier
            .fillMaxSize()
            .background(gradient),
    ) {
        Canvas(modifier = Modifier.fillMaxSize()) {
            drawCircle(
                color = Color(0x22818CF8),
                radius = 350f,
                center = Offset(size.width * 0.8f, size.height * 0.2f + 50f * phase),
            )
            drawCircle(
                color = Color(0x33EC4899),
                radius = 420f,
                center = Offset(size.width * 0.2f + 40f * phase, size.height * 0.7f),
            )
        }
        content()
    }
}

@Composable
fun LiquidGlassCard(
    modifier: Modifier = Modifier,
    cornerRadius: Dp = 24.dp,
    tint: Color = Color.White.copy(alpha = 0.12f),
    content: @Composable BoxScope.() -> Unit,
) {
    val shape = RoundedCornerShape(cornerRadius)

    Box(
        modifier = modifier
            .shadow(16.dp, shape, spotColor = Color.Black.copy(alpha = 0.3f))
            .clip(shape)
            .background(tint)
            .border(
                width = 1.dp,
                brush = Brush.verticalGradient(
                    colors = listOf(
                        Color.White.copy(alpha = 0.45f),
                        Color.White.copy(alpha = 0.1f),
                        Color.Transparent,
                    ),
                ),
                shape = shape,
            ),
    ) {
        Canvas(modifier = Modifier.fillMaxSize()) {
            drawRect(
                brush = Brush.radialGradient(
                    colors = listOf(
                        Color.White.copy(alpha = 0.15f),
                        Color.Transparent,
                    ),
                    center = Offset(size.width * 0.2f, 0f),
                    radius = size.width * 0.8f,
                ),
            )
        }
        content()
    }
}

@Composable
fun LiquidGlassButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    contentDescription: String? = null,
) {
    val shape = RoundedCornerShape(18.dp)

    Box(
        modifier = modifier
            .height(52.dp)
            .shadow(8.dp, shape)
            .clip(shape)
            .background(
                if (enabled) {
                    Brush.horizontalGradient(
                        colors = listOf(
                            Color(0x556366F1),
                            Color(0x55EC4899),
                        ),
                    )
                } else {
                    Brush.horizontalGradient(
                        colors = listOf(
                            Color.White.copy(alpha = 0.08f),
                            Color.White.copy(alpha = 0.05f),
                        ),
                    )
                },
            )
            .border(
                width = 1.dp,
                brush = Brush.verticalGradient(
                    colors = listOf(
                        Color.White.copy(alpha = 0.5f),
                        Color.White.copy(alpha = 0.15f),
                    ),
                ),
                shape = shape,
            )
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .semantics {
                if (contentDescription != null) {
                    this.contentDescription = contentDescription
                }
            }
            .padding(horizontal = 16.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = text,
            color = if (enabled) Color.White else Color.White.copy(alpha = 0.4f),
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
        )
    }
}

/**
 * Botón compacto para controles con icono. El icono nunca va sin
 * [contentDescription]: es lo que anuncia el lector de pantalla.
 */
@Composable
fun LiquidGlassIconButton(
    icon: ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    val shape = RoundedCornerShape(18.dp)

    Box(
        modifier = modifier
            .width(64.dp)
            .height(56.dp)
            .shadow(8.dp, shape)
            .clip(shape)
            .background(Color.White.copy(alpha = 0.12f))
            .border(
                width = 1.dp,
                brush = Brush.verticalGradient(
                    colors = listOf(
                        Color.White.copy(alpha = 0.45f),
                        Color.White.copy(alpha = 0.15f),
                    ),
                ),
                shape = shape,
            )
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = icon,
            contentDescription = contentDescription,
            tint = if (enabled) Color.White else Color.White.copy(alpha = 0.4f),
            modifier = Modifier.size(26.dp),
        )
    }
}

/**
 * Interruptor de estilo propio con área táctil de 48 dp y semántica de
 * [Role.Switch] para que TalkBack lo anuncie como interruptor, no como texto.
 */
@Composable
fun LiquidGlassSwitch(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    contentDescription: String? = null,
) {
    val shape = RoundedCornerShape(50)

    Box(
        modifier = modifier
            .width(56.dp)
            .height(48.dp)
            .semantics {
                if (contentDescription != null) {
                    this.contentDescription = contentDescription
                }
            }
            .toggleable(value = checked, role = Role.Switch, onValueChange = onCheckedChange)
            .padding(vertical = 8.dp),
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .clip(shape)
                .background(
                    if (checked) {
                        Brush.horizontalGradient(
                            colors = listOf(Color(0xFF818CF8), Color(0xFFEC4899)),
                        )
                    } else {
                        Brush.horizontalGradient(
                            colors = listOf(
                                Color.White.copy(alpha = 0.18f),
                                Color.White.copy(alpha = 0.12f),
                            ),
                        )
                    },
                )
                .border(1.dp, Color.White.copy(alpha = 0.25f), shape),
        )
        Box(
            modifier = Modifier
                .align(if (checked) Alignment.CenterEnd else Alignment.CenterStart)
                .padding(horizontal = 2.dp)
                .size(24.dp)
                .clip(CircleShape)
                .background(Color.White),
        )
    }
}

/**
 * Barra de progreso arrastrable. La franja visible mide 14 dp, pero el área
 * táctil es de 48 dp y responde a toques y arrastres. Expone
 * [ProgressBarRangeInfo] y [setProgress] para accesibilidad.
 */
@Composable
fun LiquidGlassSlider(
    progress: Float,
    onProgressChange: (Float) -> Unit,
    modifier: Modifier = Modifier,
    contentDescription: String = "Posición de reproducción",
) {
    val clamped = progress.coerceIn(0f, 1f)

    Canvas(
        modifier = modifier
            .fillMaxWidth()
            .height(48.dp)
            .semantics {
                this.contentDescription = contentDescription
                progressBarRangeInfo = ProgressBarRangeInfo(clamped, 0f..1f)
                setProgress { target ->
                    onProgressChange(target.coerceIn(0f, 1f))
                    true
                }
            }
            .pointerInput(Unit) {
                detectTapGestures { offset ->
                    if (size.width > 0) {
                        onProgressChange((offset.x / size.width).coerceIn(0f, 1f))
                    }
                }
            }
            .pointerInput(Unit) {
                detectHorizontalDragGestures(
                    onDragStart = { offset ->
                        if (size.width > 0) {
                            onProgressChange((offset.x / size.width).coerceIn(0f, 1f))
                        }
                    },
                    onHorizontalDrag = { change, _ ->
                        change.consume()
                        if (size.width > 0) {
                            onProgressChange((change.position.x / size.width).coerceIn(0f, 1f))
                        }
                    },
                )
            },
    ) {
        val barHeight = 14.dp.toPx()
        val barTop = (size.height - barHeight) / 2f
        val barRadius = CornerRadius(barHeight / 2f)

        drawRoundRect(
            color = Color.White.copy(alpha = 0.15f),
            topLeft = Offset(0f, barTop),
            size = Size(size.width, barHeight),
            cornerRadius = barRadius,
        )

        if (clamped > 0f) {
            drawRoundRect(
                brush = Brush.horizontalGradient(
                    colors = listOf(Color(0xFF818CF8), Color(0xFFF472B6)),
                ),
                topLeft = Offset(0f, barTop),
                size = Size(size.width * clamped, barHeight),
                cornerRadius = barRadius,
            )
        }

        val thumbRadius = 9.dp.toPx()
        val maxThumbX = (size.width - thumbRadius).coerceAtLeast(thumbRadius)
        val thumbX = (size.width * clamped).coerceIn(thumbRadius, maxThumbX)
        drawCircle(
            color = Color.White,
            radius = thumbRadius,
            center = Offset(thumbX, size.height / 2f),
        )
        drawCircle(
            color = Color(0xFFEC4899),
            radius = thumbRadius / 2f,
            center = Offset(thumbX, size.height / 2f),
        )
    }
}
