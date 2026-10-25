package com.kyant.liquidglass.music

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val prefs = getSharedPreferences("liquid_glass_prefs", Context.MODE_PRIVATE)

        setContent {
            var onboardingCompleted by rememberSaveable {
                mutableStateOf(prefs.getBoolean("onboarding_completed", false))
            }
            var autoplay by rememberSaveable {
                mutableStateOf(prefs.getBoolean("autoplay", true))
            }

            if (!onboardingCompleted) {
                OnboardingView(
                    initialAutoplay = autoplay,
                    onFinishOnboarding = { chosenAutoplay ->
                        prefs.edit()
                            .putBoolean("onboarding_completed", true)
                            .putBoolean("autoplay", chosenAutoplay)
                            .apply()
                        autoplay = chosenAutoplay
                        onboardingCompleted = true
                    },
                )
            } else {
                MusicPlayerView(
                    autoplay = autoplay,
                    onResetOnboarding = {
                        prefs.edit().putBoolean("onboarding_completed", false).apply()
                        onboardingCompleted = false
                    },
                )
            }
        }
    }
}
