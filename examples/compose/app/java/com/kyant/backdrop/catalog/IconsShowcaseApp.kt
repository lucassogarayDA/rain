package com.kyant.backdrop.catalog

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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import kotlinx.coroutines.launch
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

data class IconItem(
    val id: String,
    val name: String,
    val color: Color = Color(0xFFFF5722),
)

val iconList = listOf(
    IconItem(id = "fire", name = "FireIcon"),
    IconItem(id = "flame", name = "FlameIcon"),
    IconItem(id = "cloudsync", name = "CloudSyncIcon"),
    IconItem(id = "bot", name = "BotIcon"),
    IconItem(id = "mochi", name = "Mochi"),
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun IconsShowcaseApp() {
    var showBottomSheet by remember { mutableStateOf(false) }
    var selectedIconId by remember { mutableStateOf("fire") }
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)

    androidx.compose.runtime.LaunchedEffect(showBottomSheet) {
        if (showBottomSheet) {
            sheetState.show()
        } else {
            sheetState.hide()
        }
    }

    MaterialTheme {
        Surface(
            modifier = Modifier.fillMaxSize(),
            color = Color(0xFF0F1026),
        ) {
            GalleryScreen(
                onIconClick = { iconId ->
                    selectedIconId = iconId
                    showBottomSheet = true
                },
            )

            if (showBottomSheet) {
                ModalBottomSheet(
                    onDismissRequest = { showBottomSheet = false },
                    sheetState = sheetState,
                    containerColor = Color(0xFF0F1026),
                    scrimColor = Color.Black.copy(alpha = 0.6f),
                ) {
                    IconDetailBottomSheet(
                        iconId = selectedIconId,
                        onClose = { showBottomSheet = false },
                        sheetState = sheetState,
                    )
                }
            }
        }
    }
}

@Composable
fun GalleryScreen(
    onIconClick: (String) -> Unit,
) {
    var searchQuery by remember { mutableStateOf("") }

    val filteredIcons = remember(searchQuery) {
        if (searchQuery.isBlank()) {
            iconList
        } else {
            iconList.filter {
                it.name.contains(searchQuery, ignoreCase = true)
            }
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp),
    ) {
        Text(
            text = "Icon Catalog",
            color = Color.White,
            fontSize = 28.sp,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(top = 16.dp, bottom = 16.dp),
        )
        OutlinedTextField(
            value = searchQuery,
            onValueChange = { searchQuery = it },
            modifier = Modifier.fillMaxWidth(),
            placeholder = { Text("Buscar icono...", color = Color.White.copy(alpha = 0.5f)) },
            textStyle = androidx.compose.ui.text.TextStyle(color = Color.White),
        )
        Spacer(modifier = Modifier.height(16.dp))
        LazyVerticalGrid(
            columns = GridCells.Fixed(2),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            items(filteredIcons) { item ->
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(120.dp)
                        .clip(RoundedCornerShape(16.dp))
                        .background(Color(0xFF1E1E2E))
                        .clickable { onIconClick(item.id) }
                        .padding(16.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        when (item.id) {
                            "flame" -> FlameIcon(size = 40.dp, color = item.color)
                            "cloudsync" -> CloudSyncIcon(size = 40.dp, color = item.color)
                            "bot" -> BotIcon(size = 40.dp, color = item.color)
                            "mochi" -> FireIconFlat(size = 40.dp)
                            else -> FireIcon(size = 40.dp, color = item.color)
                        }
                        Text(
                            text = item.name,
                            color = Color.White,
                            fontSize = 12.sp,
                        )
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun IconDetailBottomSheet(
    iconId: String,
    onClose: () -> Unit,
    sheetState: androidx.compose.material3.SheetState? = null,
) {
    val fireState = rememberFireIconState()
    val flameState = rememberFlameIconState()
    val cloudSyncState = rememberCloudSyncIconState()
    val botState = rememberBotIconState()

    var iconSize by remember { mutableFloatStateOf(64f) }
    var animationDuration by remember { mutableFloatStateOf(1f) }
    var selectedColor by remember { mutableStateOf(Color(0xFFFF5722)) }

    val presetColors = listOf(
        Color(0xFFFF5722),
        Color(0xFFFF9800),
        Color(0xFFFFEB3B),
        Color(0xFF4CAF50),
        Color(0xFF2196F3),
        Color(0xFF9C27B0),
        Color(0xFFE91E63),
        Color(0xFFFFFFFF),
        Color(0xFF90A4AE),
    )

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp)
            .padding(bottom = 32.dp),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(bottom = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = when (iconId) {
                    "flame" -> "FlameIcon"
                    "cloudsync" -> "CloudSyncIcon"
                    "bot" -> "BotIcon"
                    "mochi" -> "Mochi"
                    else -> "FireIcon"
                },
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
            )
            Spacer(modifier = Modifier.weight(1f))
            val scope = rememberCoroutineScope()
            Button(onClick = {
                scope.launch {
                    sheetState?.hide()
                    onClose()
                }
            }) {
                Text("Cerrar")
            }
        }

        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(200.dp)
                .clip(RoundedCornerShape(20.dp))
                .background(Color(0xFF1E1E2E))
                .clickable {
                    when (iconId) {
                        "flame" -> flameState.startAnimation()
                        "cloudsync" -> cloudSyncState.startAnimation()
                        "bot" -> botState.startAnimation()
                        else -> if (iconId != "mochi") fireState.startAnimation()
                    }
                },
            contentAlignment = Alignment.Center,
        ) {
            when (iconId) {
                "flame" ->
                    FlameIcon(
                        size = iconSize.dp,
                        color = selectedColor,
                        duration = animationDuration,
                        state = flameState,
                    )
                "cloudsync" ->
                    CloudSyncIcon(
                        size = iconSize.dp,
                        color = selectedColor,
                        duration = animationDuration,
                        state = cloudSyncState,
                    )
                "bot" ->
                    BotIcon(
                        size = iconSize.dp,
                        color = selectedColor,
                        duration = animationDuration,
                        state = botState,
                    )
                "mochi" -> FireIconFlat(size = iconSize.dp)
                else ->
                    FireIcon(
                        size = iconSize.dp,
                        color = selectedColor,
                        duration = animationDuration,
                        state = fireState,
                    )
            }
        }

        Text(
            text = "Toca el icono para animar",
            color = Color.White.copy(alpha = 0.6f),
            fontSize = 12.sp,
            modifier = Modifier
                .align(Alignment.CenterHorizontally)
                .padding(top = 8.dp),
        )

        Spacer(modifier = Modifier.height(24.dp))

        Text(
            text = "Controles",
            color = Color.White,
            fontSize = 18.sp,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(bottom = 12.dp),
        )

        Row(
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.fillMaxWidth(),
        ) {
            Button(
                onClick = {
                    when (iconId) {
                        "flame" -> flameState.startAnimation()
                        "cloudsync" -> cloudSyncState.startAnimation()
                        "bot" -> botState.startAnimation()
                        else -> if (iconId != "mochi") fireState.startAnimation()
                    }
                },
                modifier = Modifier.weight(1f),
                enabled = iconId != "mochi",
            ) {
                Text("Iniciar")
            }
            Button(
                onClick = {
                    when (iconId) {
                        "flame" -> flameState.stopAnimation()
                        "cloudsync" -> cloudSyncState.stopAnimation()
                        "bot" -> botState.stopAnimation()
                        else -> if (iconId != "mochi") fireState.stopAnimation()
                    }
                },
                modifier = Modifier.weight(1f),
                enabled = iconId != "mochi",
            ) {
                Text("Detener/Pausar")
            }
        }

        Spacer(modifier = Modifier.height(24.dp))

        if (iconId != "mochi") {
            Text(
                text = "Color",
                color = Color.White,
                fontSize = 16.sp,
                modifier = Modifier.padding(bottom = 8.dp),
            )
            Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth(),
            ) {
                presetColors.forEach { color ->
                    Box(
                        modifier = Modifier
                            .size(40.dp)
                            .clip(CircleShape)
                            .background(color)
                            .clickable { selectedColor = color },
                    )
                }
            }
            Spacer(modifier = Modifier.height(24.dp))
        } else {
            Spacer(modifier = Modifier.height(8.dp))
        }

        Text(
            text = "Tamaño: ${iconSize.toInt()} dp (16 - 128)",
            color = Color.White,
            fontSize = 16.sp,
        )
        Slider(
            value = iconSize,
            onValueChange = { iconSize = it },
            valueRange = 16f..128f,
            modifier = Modifier.fillMaxWidth(),
        )

        if (iconId != "mochi") {
            Spacer(modifier = Modifier.height(16.dp))

            Text(
                text = "Duración animación: ${String.format("%.1f", animationDuration)}s (max 3.0s)",
                color = Color.White,
                fontSize = 16.sp,
            )
            Slider(
                value = animationDuration,
                onValueChange = { animationDuration = it },
                valueRange = 0.1f..3.0f,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}
