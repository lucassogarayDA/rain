package com.kyant.backdrop.catalog

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun FireIconBasicExample() {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF2A2A3C)),
    ) {
        Column(
            modifier = Modifier.padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = "Icono Básico",
                color = Color.White.copy(alpha = 0.7f),
                fontSize = 14.sp,
                modifier = Modifier.padding(bottom = 12.dp),
            )
            FireIcon(
                size = 48.dp,
                color = Color(0xFFFF5722),
            )
        }
    }
}

@Composable
fun FireIconButtonExample() {
    val fireState = rememberFireIconState()

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF2A2A3C)),
    ) {
        Column(
            modifier = Modifier.padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                text = "IconButton Interactivo",
                color = Color.White.copy(alpha = 0.7f),
                fontSize = 14.sp,
            )
            IconButton(
                onClick = { fireState.startAnimation() },
            ) {
                FireIcon(
                    size = 40.dp,
                    color = Color(0xFFFF7043),
                    state = fireState,
                )
            }
            Text(
                text = "Toca el icono para animar",
                color = Color.White,
                fontSize = 14.sp,
            )
        }
    }
}

@Composable
fun FireIconControlsExample() {
    val fireState = rememberFireIconState()

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF2A2A3C)),
    ) {
        Column(
            modifier = Modifier.padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(
                text = "Controles de Animación",
                color = Color.White.copy(alpha = 0.7f),
                fontSize = 14.sp,
            )
            FireIcon(
                size = 56.dp,
                color = Color(0xFFFF5722),
                state = fireState,
            )

            Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Button(
                    onClick = { fireState.startAnimation() },
                ) {
                    Text("Iniciar")
                }

                Button(
                    onClick = { fireState.stopAnimation() },
                ) {
                    Text("Detener")
                }
            }
        }
    }
}

@Composable
fun FireIconDemo() {
    MaterialTheme {
        Surface(
            modifier = Modifier.fillMaxSize(),
            color = Color(0xFF1E1E2E),
        ) {
            Column(
                modifier =
                    Modifier
                        .fillMaxSize()
                        .padding(24.dp)
                        .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(20.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    text = "Prueba de FireIcon 🔥",
                    color = Color.White,
                    fontSize = 26.sp,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(top = 16.dp, bottom = 8.dp),
                )

                FireIconBasicExample()

                FireIconButtonExample()

                FireIconControlsExample()

                FlameIconDemo()
            }
        }
    }
}

@Composable
fun FlameIconDemo() {
    val flameState = rememberFlameIconState()

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF2A2A3C)),
    ) {
        Column(
            modifier = Modifier.padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(
                text = "FlameIcon",
                color = Color.White.copy(alpha = 0.7f),
                fontSize = 14.sp,
            )
            FlameIcon(
                size = 56.dp,
                color = Color(0xFFFF5722),
                state = flameState,
            )
            Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Button(
                    onClick = { flameState.startAnimation() },
                ) {
                    Text("Iniciar")
                }
                Button(
                    onClick = { flameState.stopAnimation() },
                ) {
                    Text("Detener")
                }
            }
            Row(
                horizontalArrangement = Arrangement.spacedBy(16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                FlameIcon(
                    size = 32.dp,
                    color = Color(0xFFFF7043),
                )
                IconButton(
                    onClick = { },
                ) {
                    FlameIcon(
                        size = 28.dp,
                        color = Color(0xFFFF5722),
                    )
                }
            }
        }
    }
}
