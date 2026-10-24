package com.kyant.liquidglass.music

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun OnboardingView(
    onFinishOnboarding: () -> Unit,
) {
    var step by remember { mutableIntStateOf(1) }
    var permissionsGranted by remember { mutableIntStateOf(0) }
    val selectedFolders = remember { mutableStateListOf("/storage/emulated/0/Music", "/storage/emulated/0/Download") }

    LiquidGlassBackground {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween,
        ) {
            Column(
                modifier = Modifier.padding(top = 32.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    text = "Liquid Glass Music",
                    color = Color.White,
                    fontSize = 28.sp,
                    fontWeight = FontWeight.Bold,
                )
                Text(
                    text = "Paso $step de 3",
                    color = Color.White.copy(alpha = 0.6f),
                    fontSize = 14.sp,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }

            AnimatedContent(
                targetState = step,
                transitionSpec = { fadeIn() togetherWith fadeOut() },
                label = "onboardingStep",
            ) { currentStep ->
                when (currentStep) {
                    1 -> StepPermissions(
                        isGranted = permissionsGranted == 1,
                        onRequestPermission = {
                            permissionsGranted = 1
                            step = 2
                        },
                    )
                    2 -> StepFolderSelection(
                        selectedFolders = selectedFolders,
                        onToggleFolder = { folder ->
                            if (selectedFolders.contains(folder)) {
                                selectedFolders.remove(folder)
                            } else {
                                selectedFolders.add(folder)
                            }
                        },
                    )
                    3 -> StepSummary(
                        folderCount = selectedFolders.size,
                    )
                }
            }

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 24.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                if (step > 1) {
                    LiquidGlassButton(
                        text = "Atrás",
                        onClick = { step-- },
                        modifier = Modifier.weight(1f).padding(end = 8.dp),
                    )
                } else {
                    Spacer(modifier = Modifier.weight(1f))
                }

                if (step < 3) {
                    LiquidGlassButton(
                        text = "Siguiente",
                        enabled = true,
                        onClick = {
                            if (step == 1) {
                                permissionsGranted = 1
                            }
                            step++
                        },
                        modifier = Modifier.weight(1f).padding(start = 8.dp),
                    )
                } else {
                    LiquidGlassButton(
                        text = "Comenzar",
                        onClick = onFinishOnboarding,
                        modifier = Modifier.weight(1f).padding(start = 8.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun StepPermissions(
    isGranted: Boolean,
    onRequestPermission: () -> Unit,
) {
    LiquidGlassCard(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 16.dp),
    ) {
        Column(
            modifier = Modifier.padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = "🔐 Permisos de Almacenamiento",
                color = Color.White,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = "Para reproducir tus canciones locales con la experiencia Liquid Glass, necesitamos acceder a tus archivos de audio.",
                color = Color.White.copy(alpha = 0.8f),
                fontSize = 15.sp,
                textAlign = TextAlign.Center,
            )
            Spacer(modifier = Modifier.height(24.dp))
            LiquidGlassButton(
                text = if (isGranted) "✅ Permisos Otorgados" else "Conceder Permiso y Continuar",
                enabled = true,
                onClick = onRequestPermission,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@Composable
private fun StepFolderSelection(
    selectedFolders: List<String>,
    onToggleFolder: (String) -> Unit,
) {
    val availableFolders = listOf(
        "/storage/emulated/0/Music",
        "/storage/emulated/0/Download",
        "/storage/emulated/0/Audio",
        "/storage/emulated/0/Podcasts",
    )

    LiquidGlassCard(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 16.dp),
    ) {
        Column(
            modifier = Modifier.padding(24.dp),
        ) {
            Text(
                text = "📁 Carpetas de Canciones",
                color = Color.White,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "Selecciona las carpetas donde guardas tus archivos MP3 / FLAC:",
                color = Color.White.copy(alpha = 0.8f),
                fontSize = 14.sp,
            )
            Spacer(modifier = Modifier.height(16.dp))

            availableFolders.forEach { folder ->
                val isSelected = selectedFolders.contains(folder)
                LiquidGlassCard(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 4.dp)
                        .clickable { onToggleFolder(folder) },
                    cornerRadius = 12.dp,
                    tint = if (isSelected) Color(0x336366F1) else Color.White.copy(alpha = 0.05f),
                ) {
                    Row(
                        modifier = Modifier.padding(14.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            text = if (isSelected) "☑️" else "⬛",
                            modifier = Modifier.padding(end = 12.dp),
                        )
                        Text(
                            text = folder.substringAfterLast('/'),
                            color = Color.White,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun StepSummary(
    folderCount: Int,
) {
    LiquidGlassCard(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 16.dp),
    ) {
        Column(
            modifier = Modifier.padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = "✨ ¡Todo Listo!",
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = "Has configurado $folderCount carpeta(s) de música. Disfruta de la reproducción en alta fidelidad con efectos de cristal líquido.",
                color = Color.White.copy(alpha = 0.85f),
                fontSize = 15.sp,
                textAlign = TextAlign.Center,
            )
        }
    }
}
