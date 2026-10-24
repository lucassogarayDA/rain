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
import androidx.compose.ui.MotionDurationScale
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlin.math.PI

class CloudSyncIconState internal constructor() {
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
fun rememberCloudSyncIconState(): CloudSyncIconState {
    return remember { CloudSyncIconState() }
}

private val CloudPath: Path = PathParser().parsePathString(
    "M20.996 15.251A4.5 4.5 0 0 0 17.495 8h-1.79a7 7 0 1 0-12.709 5.607"
).toPath()

private val SyncPath1: Path = PathParser().parsePathString(
    "m17 18-1.535 1.605a5 5 0 0 1-8-1.5"
).toPath()

private val SyncPath2: Path = PathParser().parsePathString(
    "M17 22v-4h-4"
).toPath()

private val SyncPath3: Path = PathParser().parsePathString(
    "M7 10v4h4"
).toPath()

private val SyncPath4: Path = PathParser().parsePathString(
    "m7 14 1.535-1.605a5 5 0 0 1 8 1.5"
).toPath()

@Composable
fun CloudSyncIcon(
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
    color: Color = Color.Unspecified,
    isAnimated: Boolean = true,
    duration: Float = 1f,
    state: CloudSyncIconState? = null,
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
                    animationSpec = tween(
                        durationMillis = (900 * duration.coerceAtLeast(0.1f)).toInt(),
                    ),
                )
                progress.snapTo(0f)
            }
        }
    }

    val stop: () -> Unit = remember {
        {
            animationJob?.cancel()
            animationJob = scope.launch {
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

    val resolvedColor = remember(color) {
        if (color == Color.Unspecified) Color(0xFF90A4AE) else color
    }

    Canvas(
        modifier = modifier
            .size(size)
            .hoverable(
                interactionSource = interactionSource,
                enabled = isAnimated,
            ),
    ) {
        val sx = this.size.width / 24f
        val sy = this.size.height / 24f
        val phase = progress.value
        val rotation = 180f * phase
        val scaleAnim = 1f + (if (phase < 0.5f) -0.08f * (phase * 2f) else -0.08f * ((1f - phase) * 2f))

        scale(sx, sy, pivot = Offset.Zero) {
            drawPath(
                path = CloudPath,
                color = resolvedColor,
                style = Stroke(
                    width = 2f,
                    cap = StrokeCap.Round,
                    join = StrokeJoin.Round,
                ),
            )
            rotate(
                degrees = rotation,
                pivot = Offset(12f, 16f),
            ) {
                scale(
                    scaleX = scaleAnim,
                    scaleY = scaleAnim,
                    pivot = Offset(12f, 16f),
                ) {
                    drawPath(
                        path = SyncPath1,
                        color = resolvedColor,
                        style = Stroke(
                            width = 2f,
                            cap = StrokeCap.Round,
                            join = StrokeJoin.Round,
                        ),
                    )
                    drawPath(
                        path = SyncPath2,
                        color = resolvedColor,
                        style = Stroke(
                            width = 2f,
                            cap = StrokeCap.Round,
                            join = StrokeJoin.Round,
                        ),
                    )
                    drawPath(
                        path = SyncPath3,
                        color = resolvedColor,
                        style = Stroke(
                            width = 2f,
                            cap = StrokeCap.Round,
                            join = StrokeJoin.Round,
                        ),
                    )
                    drawPath(
                        path = SyncPath4,
                        color = resolvedColor,
                        style = Stroke(
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
