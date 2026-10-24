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
import androidx.compose.ui.MotionDurationScale
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlin.math.PI
import kotlin.math.sin

class FireIconState internal constructor() {
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
fun rememberFireIconState(): FireIconState = remember { FireIconState() }

private val FirePath: Path =
    PathParser()
        .parsePathString(
            "M13.8561 22C26.0783 19 19.2338 7 10.9227 2 " +
                "C9.9453 5.5 8.47838 6.5 5.54497 10 " +
                "C1.66121 14.6339 3.5895 20 8.96719 22 " +
                "C8.1524 21 6.04958 18.9008 7.5 16 " +
                "C8 15 9 14 8.5 12 " +
                "C9.47778 12.5 11.5 13 12 15.5 " +
                "C12.8148 14.5 13.6604 12.4 12.8783 10 " +
                "C19 14.5 16.5 19 13.8561 22Z",
        ).toPath()

@Composable
fun FireIcon(
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
    color: Color = Color.Unspecified,
    isAnimated: Boolean = true,
    duration: Float = 1f,
    state: FireIconState? = null,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val isHovered by interactionSource.collectIsHoveredAsState()
    val progress = remember { Animatable(0f) }
    val scope = rememberCoroutineScope()
    val motionScale = scope.coroutineContext[MotionDurationScale]?.scaleFactor ?: 1f

    var animationJob by remember { mutableStateOf<Job?>(null) }

    val start: () -> Unit =
        remember(motionScale, duration) {
            {
                if (motionScale <= 0f) return@remember
                animationJob?.cancel()
                animationJob =
                    scope.launch {
                        progress.snapTo(0f)
                        progress.animateTo(
                            targetValue = 1f,
                            animationSpec =
                                tween(
                                    durationMillis = (800 * duration.coerceAtLeast(0.1f)).toInt(),
                                ),
                        )
                        progress.snapTo(0f)
                    }
            }
        }

    val stop: () -> Unit =
        remember {
            {
                animationJob?.cancel()
                animationJob =
                    scope.launch {
                        progress.snapTo(0f)
                    }
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

    val resolvedColor =
        remember(color) {
            if (color == Color.Unspecified) Color(0xFFFF5722) else color
        }

    Canvas(
        modifier =
            modifier
                .size(size)
                .hoverable(
                    interactionSource = interactionSource,
                    enabled = isAnimated,
                ),
    ) {
        val sx = this.size.width / 24f
        val sy = this.size.height / 24f
        val phase = progress.value
        val scaleX = 1f + 0.07f * sin(phase * PI * 2).toFloat()
        val scaleY = 1f + 0.12f * sin(phase * PI * 2).toFloat()
        val rotation = -4f * sin(phase * PI * 2).toFloat()

        scale(sx, sy, pivot = Offset.Zero) {
            scale(
                scaleX = scaleX,
                scaleY = scaleY,
                pivot = Offset(12f, 22f),
            ) {
                rotate(
                    degrees = rotation,
                    pivot = Offset(12f, 22f),
                ) {
                    drawPath(
                        path = FirePath,
                        color = resolvedColor,
                        style =
                            Stroke(
                                width = 2f,
                                cap = StrokeCap.Round,
                                join = StrokeJoin.Round,
                            ),
                    )
                }
            }
        }
    }
}
