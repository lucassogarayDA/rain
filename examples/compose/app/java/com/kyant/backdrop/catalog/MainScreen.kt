package com.kyant.backdrop.catalog

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.unit.dp

@Composable
fun MainScreen() {
    Button(
        onClick = {},
        shape = RoundedCornerShape(
            topStart = 24.dp,
            topEnd = 8.dp,
            bottomEnd = 24.dp,
            bottomStart = 8.dp
        )
    ) {
        Text("Presióname")
    }
}
