package com.kyant.liquidglass.music

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.CheckCircle
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private const val TOTAL_STEPS = 3

@Composable
fun OnboardingView(
    initialAutoplay: Boolean,
    onFinishOnboarding: (Boolean) -> Unit,
) {
    var step by rememberSaveable { mutableIntStateOf(0) }
    var autoplay by rememberSaveable { mutableStateOf(initialAutoplay) }

    LiquidGlassBackground {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "Liquid Glass Music",
                        color = Color.White,
                        fontSize = 26.sp,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(
                        text = "Demo de Rain con Jetpack Compose",
                        color = Color.White.copy(alpha = 0.6f),
                        fontSize = 13.sp,
                    )
                }
                Text(
                    text = "Saltar",
                    color = Color.White.copy(alpha = 0.75f),
                    fontSize = 14.sp,
                    modifier = Modifier
                        .clip(RoundedCornerShape(12.dp))
                        .clickable(
                            role = Role.Button,
                            onClickLabel = "Saltar la introducción",
                            onClick = { onFinishOnboarding(autoplay) },
                        )
                        .padding(horizontal = 14.dp, vertical = 16.dp),
                )
            }

            StepDots(current = step)

            Column(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.Center,
            ) {
                AnimatedContent(
                    targetState = step,
                    transitionSpec = { fadeIn() togetherWith fadeOut() },
                    label = "onboardingStep",
                ) { currentStep ->
                    when (currentStep) {
                        0 -> StepWelcome()
                        1 -> StepPlayback(
                            autoplay = autoplay,
                            onAutoplayChange = { autoplay = it },
                        )
                        else -> StepSummary(autoplay = autoplay)
                    }
                }
            }

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                if (step > 0) {
                    LiquidGlassButton(
                        text = "Atrás",
                        onClick = { step-- },
                        modifier = Modifier.weight(1f),
                    )
                } else {
                    Spacer(modifier = Modifier.weight(1f))
                }

                if (step < TOTAL_STEPS - 1) {
                    LiquidGlassButton(
                        text = "Siguiente",
                        onClick = { step++ },
                        modifier = Modifier.weight(1f),
                    )
                } else {
                    LiquidGlassButton(
                        text = "Empezar",
                        onClick = { onFinishOnboarding(autoplay) },
                        modifier = Modifier.weight(1f),
                    )
                }
            }
        }
    }
}

@Composable
private fun StepDots(current: Int) {
    Row(
        modifier = Modifier
            .padding(vertical = 16.dp)
            .semantics { contentDescription = "Paso ${current + 1} de $TOTAL_STEPS" },
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        repeat(TOTAL_STEPS) { index ->
            Box(
                modifier = Modifier
                    .height(8.dp)
                    .width(if (index == current) 24.dp else 8.dp)
                    .clip(CircleShape)
                    .background(
                        if (index == current) Color(0xFFF472B6) else Color.White.copy(alpha = 0.25f),
                    ),
            )
        }
    }
}

@Composable
private fun StepWelcome() {
    LiquidGlassCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(24.dp)) {
            Text(
                text = "Bienvenido",
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = "Esta demo muestra el efecto de cristal líquido con Compose y una lista de reproducción de ejemplo.",
                color = Color.White.copy(alpha = 0.85f),
                fontSize = 15.sp,
            )
            Spacer(modifier = Modifier.height(16.dp))
            Bullet("No lee tus archivos ni pide permisos de almacenamiento.")
            Bullet("No reproduce audio real: el progreso es una simulación con tiempo real.")
            Bullet("Funciona sin conexión y sin cuentas.")
        }
    }
}

@Composable
private fun StepPlayback(
    autoplay: Boolean,
    onAutoplayChange: (Boolean) -> Unit,
) {
    LiquidGlassCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(24.dp)) {
            Text(
                text = "Reproducción",
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "Elige cómo quieres empezar. Puedes volver aquí desde Ajustes en el reproductor.",
                color = Color.White.copy(alpha = 0.85f),
                fontSize = 15.sp,
            )
            Spacer(modifier = Modifier.height(20.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(modifier = Modifier.weight(1f).padding(end = 12.dp)) {
                    Text(
                        text = "Reproducir automáticamente",
                        color = Color.White,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Medium,
                    )
                    Text(
                        text = if (autoplay) "Activado" else "Desactivado",
                        color = Color.White.copy(alpha = 0.6f),
                        fontSize = 13.sp,
                    )
                }
                LiquidGlassSwitch(
                    checked = autoplay,
                    onCheckedChange = onAutoplayChange,
                    contentDescription = "Reproducir automáticamente al abrir",
                )
            }
        }
    }
}

@Composable
private fun StepSummary(autoplay: Boolean) {
    LiquidGlassCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(24.dp)) {
            Icon(
                imageVector = Icons.Rounded.CheckCircle,
                contentDescription = null,
                tint = Color(0xFFF472B6),
                modifier = Modifier.size(36.dp),
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = "Todo listo",
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = if (autoplay) {
                    "La reproducción automática está activada. La demo empezará a sonar (simulada) al abrir el reproductor."
                } else {
                    "La reproducción automática está desactivada. Pulsa Reproducir cuando quieras empezar."
                },
                color = Color.White.copy(alpha = 0.85f),
                fontSize = 15.sp,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = "La lista y las canciones son de ejemplo; no se lee ningún archivo del dispositivo.",
                color = Color.White.copy(alpha = 0.6f),
                fontSize = 13.sp,
            )
        }
    }
}

@Composable
private fun Bullet(text: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(bottom = 8.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Icon(
            imageVector = Icons.Rounded.CheckCircle,
            contentDescription = null,
            tint = Color(0xFFF472B6),
            modifier = Modifier
                .padding(top = 2.dp, end = 10.dp)
                .size(16.dp),
        )
        Text(
            text = text,
            color = Color.White.copy(alpha = 0.8f),
            fontSize = 14.sp,
            textAlign = TextAlign.Start,
        )
    }
}
