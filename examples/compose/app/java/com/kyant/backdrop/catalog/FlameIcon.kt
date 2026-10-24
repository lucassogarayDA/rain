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
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlin.math.PI
import kotlin.math.sin

class FlameIconState internal constructor() {
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
fun rememberFlameIconState(): FlameIconState = remember { FlameIconState() }

private val FlamePath: Path =
    PathParser()
        .parsePathString(
            "M12 3q1 4 4 6.5t3 5.5a1 1 0 0 1-14 0 5 5 0 0 1 1-3 1 1 0 0 0 5 0c0-2-1.5-3-1.5-5q0-2 2.5-4",
        ).toPath()

@Composable
fun FlameIcon(
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
    color: Color = Color.Unspecified,
    isAnimated: Boolean = true,
    duration: Float = 1f,
    state: FlameIconState? = null,
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
                ).graphicsLayer {
                    val phase = progress.value
                    scaleX = 1f + 0.07f * sin(phase * PI * 2).toFloat() + (-0.03f) * sin(phase * PI * 4).toFloat()
                    scaleY = 1f + 0.12f * sin(phase * PI * 2).toFloat() + (-0.05f) * sin(phase * PI * 4).toFloat()
                    rotationZ = (-5f * sin(phase * PI * 2).toFloat() + 4f * sin(phase * PI * 4).toFloat())
                    transformOrigin = TransformOrigin(0.5f, 0.875f)
                },
    ) {
        val sx = this.size.width / 24f
        val sy = this.size.height / 24f

        scale(sx, sy, pivot = Offset.Zero) {
            drawPath(
                path = FlamePath,
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
