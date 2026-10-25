package com.kyant.backdrop.catalog

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.hoverable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.MotionDurationScale
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

class BotIconState internal constructor() {
    internal var startAction: (() -> Unit)? = null
    internal var stopAction: (() -> Unit)? = null

    fun startAnimation() {
        startAction?.invoke()
    }

    fun stopAnimation() {
        stopAction?.invoke()
    }
}

@Composable
fun rememberBotIconState(): BotIconState {
    return remember { BotIconState() }
}

private val PathAntenna: Path = PathParser().parsePathString("M12 8V4H8").toPath()
private val PathLeftLine: Path = PathParser().parsePathString("M2 14h2").toPath()
private val PathRightLine: Path = PathParser().parsePathString("M20 14h2").toPath()
private val PathEyeLeft: Path = PathParser().parsePathString("M9 13v2").toPath()
private val PathEyeRight: Path = PathParser().parsePathString("M15 13v2").toPath()

@Composable
fun BotIcon(
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
    color: Color = Color.Unspecified,
    isAnimated: Boolean = true,
    duration: Float = 1f,
    state: BotIconState? = null,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val isHovered by interactionSource.collectIsHoveredAsState()
    val progress = remember { Animatable(0f) }
    val scope = rememberCoroutineScope()
    val motionScale = scope.coroutineContext[MotionDurationScale]?.scaleFactor ?: 1f

    var animationJob by remember { mutableStateOf<Job?>(null) }

    val start: () -> Unit = remember(motionScale, duration) {
        {
            if (motionScale <= 0f) return@remember
            animationJob?.cancel()
            animationJob = scope.launch {
                progress.snapTo(0f)
                progress.animateTo(
                    targetValue = 1f,
                    animationSpec = tween(durationMillis = (800 * duration.coerceAtLeast(0.1f)).toInt()),
                )
                progress.snapTo(0f)
            }
        }
    }

    val stop: () -> Unit = remember {
        {
            animationJob?.cancel()
            animationJob = scope.launch { progress.snapTo(0f) }
        }
    }

    DisposableEffect(state) {
        if (state != null) {
            state.startAction = start
            state.stopAction = stop
        }
        onDispose {
            if (state != null) {
                state.startAction = null
                state.stopAction = null
            }
            animationJob?.cancel()
            animationJob = null
        }
    }

    LaunchedEffect(isHovered, isAnimated) {
        if (isAnimated && isHovered) {
            start()
        } else {
            stop()
        }
    }

    val resolvedColor = remember(color) { if (color == Color.Unspecified) Color.White else color }
    val p = progress.value

    val headTranslateY = when {
        p < 0.25f -> -1.5f * (p / 0.25f)
        p < 0.5f -> -1.5f * (1f - (p - 0.25f) / 0.25f)
        p < 0.75f -> -0.75f * ((p - 0.5f) / 0.25f)
        else -> -0.75f * (1f - (p - 0.75f) / 0.25f)
    }
    val antennaRot = when {
        p < 0.25f -> -16f * (p / 0.25f)
        p < 0.5f -> -16f * (1f - (p - 0.25f) / 0.25f)
        p < 0.75f -> 12f * ((p - 0.5f) / 0.25f)
        else -> 12f * (1f - (p - 0.75f) / 0.25f)
    }
    val eyeScaleY = when {
        p < 0.33f -> 1f - 0.9f * (p / 0.33f)
        p < 0.67f -> 0.1f
        else -> 0.1f + 0.9f * ((p - 0.67f) / 0.33f)
    }

    Canvas(
        modifier = modifier
            .size(size)
            .hoverable(interactionSource = interactionSource, enabled = isAnimated),
    ) {
        val sx = this.size.width / 24f
        val sy = this.size.height / 24f
        scale(sx, sy, pivot = Offset.Zero) {
            translate(top = headTranslateY) {
                rotate(antennaRot, pivot = Offset(12f, 8f)) {
                    drawPath(PathAntenna, resolvedColor, style = Stroke(width = 2f, cap = StrokeCap.Round, join = StrokeJoin.Round))
                }
                drawRoundRect(resolvedColor, topLeft = Offset(4f, 8f), size = Size(16f, 12f), cornerRadius = CornerRadius(2f, 2f), style = Stroke(width = 2f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            }
            drawPath(PathLeftLine, resolvedColor, style = Stroke(width = 2f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            drawPath(PathRightLine, resolvedColor, style = Stroke(width = 2f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            scale(scaleX = 1f, scaleY = eyeScaleY, pivot = Offset(9f, 14f)) {
                drawPath(PathEyeLeft, resolvedColor, style = Stroke(width = 2f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            }
            scale(scaleX = 1f, scaleY = eyeScaleY, pivot = Offset(15f, 14f)) {
                drawPath(PathEyeRight, resolvedColor, style = Stroke(width = 2f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            }
        }
    }
}
