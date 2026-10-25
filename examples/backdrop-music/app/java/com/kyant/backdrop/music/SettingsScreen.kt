package com.kyant.backdrop.music

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.DeleteSweep
import androidx.compose.material.icons.rounded.GraphicEq
import androidx.compose.material.icons.rounded.LibraryMusic
import androidx.compose.material.icons.rounded.Palette
import androidx.compose.material.icons.rounded.PlayArrow
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.RestartAlt
import androidx.compose.material.icons.rounded.Storage
import androidx.compose.material3.Icon
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kyant.backdrop.Backdrop

/**
 * Configuración real de la app. Cada sección agrupa ajustes que se aplican en
 * el momento y se guardan en `SharedPreferences`: audio, apariencia,
 * biblioteca, reproducción y almacenamiento.
 */
@Composable
fun SettingsScreen(
    settings: MusicSettings,
    controller: PlayerController,
    tracks: List<Track>,
    backdrop: Backdrop,
    accent: Color,
    onRescan: () -> Unit,
    onOpenEqualizer: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var message by remember { mutableStateOf<String?>(null) }

    val folders = remember(tracks) { tracks.map { it.folder }.distinct().size }
    val albums = remember(tracks) { tracks.map { it.albumId to it.album }.distinct().size }
    val totalBytes = remember(tracks) { tracks.sumOf { it.sizeBytes } }

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 6.dp, bottom = 20.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item(key = "audio") {
            SettingsSection(
                title = "Audio",
                icon = Icons.Rounded.GraphicEq,
                accent = accent,
                backdrop = backdrop,
            ) {
                SettingSwitch(
                    title = "Pausar al perder el foco",
                    description = "Detiene la música cuando otra app toma el audio; " +
                        "se reanuda al recuperarlo.",
                    checked = settings.pauseOnFocusLoss,
                    accent = accent,
                    onCheckedChange = settings::updatePauseOnFocusLoss,
                )
                SettingDivider()
                ActionRow(
                    title = "Ecualizador",
                    description = when {
                        controller.hasEqualizer ->
                            "${controller.eqBands.size} bandas listas en el reproductor."
                        controller.eqError != null -> controller.eqError.orEmpty()
                        else -> "Disponible al reproducir una canción."
                    },
                    actionLabel = "Abrir",
                    icon = Icons.Rounded.GraphicEq,
                    accent = accent,
                    onClick = onOpenEqualizer,
                )
            }
        }

        item(key = "apariencia") {
            SettingsSection(
                title = "Apariencia",
                icon = Icons.Rounded.Palette,
                accent = accent,
                backdrop = backdrop,
            ) {
                SettingSwitch(
                    title = "Acento dinámico",
                    description = "El color de la interfaz se extrae de la carátula " +
                        "de la canción actual.",
                    checked = settings.dynamicAccent,
                    accent = accent,
                    onCheckedChange = settings::updateDynamicAccent,
                )
                SettingDivider()
                SettingSwitch(
                    title = "Reducir movimiento",
                    description = "Acorta las transiciones y detiene los halos animados " +
                        "del fondo.",
                    checked = settings.reduceMotion,
                    accent = accent,
                    onCheckedChange = settings::updateReduceMotion,
                )
            }
        }

        item(key = "biblioteca") {
            SettingsSection(
                title = "Biblioteca",
                icon = Icons.Rounded.LibraryMusic,
                accent = accent,
                backdrop = backdrop,
            ) {
                Text(
                    text = "Orden de la cola",
                    color = Color.White,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                )
                Spacer(Modifier.height(2.dp))
                LibrarySort.values().forEach { option ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(min = 44.dp)
                            .clip(RoundedCornerShape(14.dp))
                            .selectable(
                                selected = settings.sort == option,
                                role = Role.RadioButton,
                                onClick = { settings.updateSort(option) },
                            ),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(
                            selected = settings.sort == option,
                            onClick = null,
                            colors = RadioButtonDefaults.colors(
                                selectedColor = accent,
                                unselectedColor = Color.White.copy(alpha = 0.45f),
                            ),
                        )
                        Spacer(Modifier.width(6.dp))
                        Text(
                            text = option.label,
                            color = Color.White.copy(alpha = 0.88f),
                            fontSize = 13.sp,
                        )
                    }
                }

                SettingDivider()

                ActionRow(
                    title = "Volver a escanear",
                    description = "Consulta MediaStore por audio nuevo o eliminado.",
                    actionLabel = "Escanear",
                    icon = Icons.Rounded.Refresh,
                    accent = accent,
                    onClick = {
                        onRescan()
                        message = "Revisando la biblioteca…"
                    },
                )
                SettingDivider()
                ActionRow(
                    title = "Borrar caché de carátulas",
                    description = "Libera las miniaturas decodificadas en memoria. " +
                        "No borra ningún archivo de música.",
                    actionLabel = "Borrar",
                    icon = Icons.Rounded.DeleteSweep,
                    accent = accent,
                    onClick = {
                        ArtworkLoader.clearCache()
                        message = "Caché de carátulas borrada."
                    },
                )
            }
        }

        item(key = "reproduccion") {
            SettingsSection(
                title = "Reproducción",
                icon = Icons.Rounded.PlayArrow,
                accent = accent,
                backdrop = backdrop,
            ) {
                SettingSwitch(
                    title = "Reproducir al abrir",
                    description = "Empieza la primera pista de la cola al cargar la " +
                        "biblioteca.",
                    checked = settings.autoPlay,
                    accent = accent,
                    onCheckedChange = settings::updateAutoPlay,
                )
                SettingDivider()
                SettingSwitch(
                    title = "Aleatorio",
                    description = "Elige la siguiente pista al azar sin repetir la actual.",
                    checked = settings.shuffle,
                    accent = accent,
                    onCheckedChange = settings::updateShuffle,
                )
                SettingDivider()
                Text(
                    text = "Repetición",
                    color = Color.White,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                )
                Spacer(Modifier.height(6.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    RepeatMode.values().forEach { mode ->
                        ChoiceChip(
                            label = mode.label,
                            selected = settings.repeatMode == mode,
                            accent = accent,
                            modifier = Modifier.weight(1f),
                            onClick = { settings.updateRepeatMode(mode) },
                        )
                    }
                }
            }
        }

        item(key = "almacenamiento") {
            SettingsSection(
                title = "Almacenamiento",
                icon = Icons.Rounded.Storage,
                accent = accent,
                backdrop = backdrop,
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(16.dp))
                        .background(Color.White.copy(alpha = 0.06f))
                        .padding(vertical = 12.dp),
                ) {
                    StorageStat(
                        value = tracks.size.toString(),
                        label = "pistas",
                        modifier = Modifier.weight(1f),
                    )
                    StorageStat(
                        value = albums.toString(),
                        label = "álbumes",
                        modifier = Modifier.weight(1f),
                    )
                    StorageStat(
                        value = folders.toString(),
                        label = "carpetas",
                        modifier = Modifier.weight(1f),
                    )
                    StorageStat(
                        value = formatBytes(totalBytes),
                        label = "audio",
                        modifier = Modifier.weight(1f),
                    )
                }

                Spacer(Modifier.height(12.dp))

                Text(
                    text = if (tracks.isEmpty()) {
                        "No hay pistas indexadas. Añade audio al dispositivo y " +
                            "vuelve a escanear."
                    } else {
                        "La biblioteca se lee del almacenamiento del teléfono; " +
                            "Backdrop Music no sube archivos a internet."
                    },
                    color = Color.White.copy(alpha = 0.6f),
                    fontSize = 12.sp,
                )

                Spacer(Modifier.height(10.dp))

                TextButton(
                    onClick = {
                        settings.reset()
                        message = "Ajustes restablecidos."
                    },
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Icon(
                        imageVector = Icons.Rounded.RestartAlt,
                        contentDescription = null,
                        tint = Color.White.copy(alpha = 0.75f),
                        modifier = Modifier.size(17.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                    Text(
                        text = "Restablecer ajustes",
                        color = Color.White.copy(alpha = 0.75f),
                        fontSize = 13.sp,
                    )
                }

                AnimatedVisibility(
                    visible = message != null,
                    enter = fadeIn(tween(200)),
                    exit = fadeOut(tween(160)),
                ) {
                    Text(
                        text = message.orEmpty(),
                        color = accent,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Medium,
                        textAlign = TextAlign.Center,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 6.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun SettingsSection(
    title: String,
    icon: ImageVector,
    accent: Color,
    backdrop: Backdrop,
    content: @Composable () -> Unit,
) {
    GlassPanel(
        backdrop = backdrop,
        modifier = Modifier.fillMaxWidth(),
        contentPadding = PaddingValues(horizontal = 18.dp, vertical = 16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                modifier = Modifier
                    .size(34.dp)
                    .clip(CircleShape)
                    .background(accent.copy(alpha = 0.28f)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = Color.White,
                    modifier = Modifier.size(18.dp),
                )
            }
            Spacer(Modifier.width(10.dp))
            Text(
                text = title,
                color = Color.White,
                fontSize = 16.sp,
                fontWeight = FontWeight.Bold,
            )
        }
        Spacer(Modifier.height(10.dp))
        content()
    }
}

@Composable
private fun SettingSwitch(
    title: String,
    description: String,
    checked: Boolean,
    accent: Color,
    onCheckedChange: (Boolean) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                color = Color.White,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
            )
            Text(
                text = description,
                color = Color.White.copy(alpha = 0.58f),
                fontSize = 11.sp,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.width(12.dp))
        Switch(
            checked = checked,
            onCheckedChange = onCheckedChange,
            colors = SwitchDefaults.colors(
                checkedThumbColor = Color.White,
                checkedTrackColor = accent,
                uncheckedThumbColor = Color.White.copy(alpha = 0.8f),
                uncheckedTrackColor = Color.White.copy(alpha = 0.16f),
                uncheckedBorderColor = Color.White.copy(alpha = 0.3f),
            ),
        )
    }
}

@Composable
private fun ActionRow(
    title: String,
    description: String,
    actionLabel: String,
    icon: ImageVector,
    accent: Color,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp)
            .clip(RoundedCornerShape(14.dp))
            .clickable(role = Role.Button, onClick = onClick),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = accent,
            modifier = Modifier.size(20.dp),
        )
        Spacer(Modifier.width(10.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                color = Color.White,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
            )
            Text(
                text = description,
                color = Color.White.copy(alpha = 0.58f),
                fontSize = 11.sp,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.width(10.dp))
        Text(
            text = actionLabel,
            color = Color.White,
            fontSize = 12.sp,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier
                .clip(CircleShape)
                .background(Color.White.copy(alpha = 0.12f))
                .padding(horizontal = 12.dp, vertical = 7.dp),
        )
    }
}

@Composable
private fun ChoiceChip(
    label: String,
    selected: Boolean,
    accent: Color,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    Box(
        modifier = modifier
            .clip(CircleShape)
            .background(
                if (selected) accent.copy(alpha = 0.42f) else Color.White.copy(alpha = 0.09f),
            )
            .clickable(role = Role.RadioButton, onClick = onClick)
            .padding(horizontal = 8.dp, vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = label,
            color = if (selected) Color.White else Color.White.copy(alpha = 0.65f),
            fontSize = 10.sp,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center,
        )
    }
}

@Composable
private fun StorageStat(value: String, label: String, modifier: Modifier = Modifier) {
    Column(modifier = modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            color = Color.White,
            fontSize = 15.sp,
            fontWeight = FontWeight.Bold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            text = label,
            color = Color.White.copy(alpha = 0.52f),
            fontSize = 10.sp,
        )
    }
}

@Composable
private fun SettingDivider() {
    Spacer(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp)
            .height(1.dp)
            .background(Color.White.copy(alpha = 0.08f)),
    )
}

private fun formatBytes(bytes: Long): String {
    if (bytes <= 0L) return "0 MB"
    val megabytes = bytes / (1024.0 * 1024.0)
    return if (megabytes >= 1024.0) {
        val gigabytes = megabytes / 1024.0
        String.format(java.util.Locale.getDefault(), "%.1f GB", gigabytes)
    } else {
        String.format(java.util.Locale.getDefault(), "%.0f MB", megabytes)
    }
}
