package com.kyant.liquidglass.music

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val prefs = getSharedPreferences("liquid_glass_prefs", Context.MODE_PRIVATE)

        setContent {
            var onboardingCompleted by remember {
                mutableStateOf(prefs.getBoolean("onboarding_completed", false))
            }

            if (!onboardingCompleted) {
                OnboardingView(
                    onFinishOnboarding = {
                        prefs.edit().putBoolean("onboarding_completed", true).apply()
                        onboardingCompleted = true
                    },
                )
            } else {
                MusicPlayerView(
                    onResetOnboarding = {
                        prefs.edit().putBoolean("onboarding_completed", false).apply()
                        onboardingCompleted = false
                    },
                )
            }
        }
    }
}
