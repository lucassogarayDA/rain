package com.kyant.backdrop.catalog

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

private val OuterStroke: Path = PathParser().parsePathString(
    "M256 432q-92 0-114-82-16-68 44-132 8 40 34 52-6-70 42-186 8 66 54 116 64 62 56 145-14 87-116 87Z"
).toPath()

private val InnerFill: Path = PathParser().parsePathString(
    "M256 432q-92 0-114-82-16-68 44-132 8 40 34 52-6-70 42-186 8 66 54 116 64 62 56 145-14 87-116 87Z"
).toPath()

private val MiddleFill: Path = PathParser().parsePathString(
    "M256 424q-60 0-70-58-6-44 28-84 8 30 26 38 0-50 30-96 12 38 36 68 30 38 16 84-12 48-66 48Z"
).toPath()

private val CoreFill: Path = PathParser().parsePathString(
    "M256 416q-30 0-34-28-2-22 18-42 6 18 22 20 20 6 28 26 0 24-34 24Z"
).toPath()

private val Stroke1: Path = PathParser().parsePathString(
    "M256 432q-92 0-114-82-16-68 44-132 8 40 34 52-6-70 42-186 8 66 54 116 64 62 56 145-14 87-116 87Z"
).toPath()

private val Stroke2: Path = PathParser().parsePathString(
    "M256 424q-60 0-70-58-6-44 28-84 8 30 26 38 0-50 30-96 12 38 36 68 30 38 16 84-12 48-66 48Z"
).toPath()

private val Stroke3: Path = PathParser().parsePathString(
    "M256 416q-30 0-34-28-2-22 18-42 6 18 22 20 20 6 28 26 0 24-34 24Z"
).toPath()

@Composable
fun FireIconFlat(
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
) {
    Canvas(modifier = modifier.size(size)) {
        val s = this.size.width / 512f
        scale(s, s, pivot = Offset.Zero) {
            drawPath(OuterStroke, Color(0xFFE5E5E5), style = Stroke(width = 60f, join = StrokeJoin.Round))
            drawPath(InnerFill, Color.White, style = Stroke(width = 54f, join = StrokeJoin.Round))
            drawPath(InnerFill, Color(0xFFF46B55))
            drawPath(MiddleFill, Color(0xFFF4B942))
            drawPath(CoreFill, Color(0xFFF5F0DF))
            drawPath(Stroke1, Color(0xFF1E1B24), style = Stroke(width = 12f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            drawPath(Stroke2, Color(0xFF1E1B24), style = Stroke(width = 8f, cap = StrokeCap.Round, join = StrokeJoin.Round))
            drawPath(Stroke3, Color(0xFF1E1B24), style = Stroke(width = 6f, cap = StrokeCap.Round, join = StrokeJoin.Round))
        }
    }
}
