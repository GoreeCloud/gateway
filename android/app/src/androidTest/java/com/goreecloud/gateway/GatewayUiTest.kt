package com.goreecloud.gateway

import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class GatewayUiTest {
    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Test
    fun cleanLaunchIsLockedAndHasNoActiveSession() {
        composeRule.onNodeWithText("Connect securely").assertIsDisplayed()
        composeRule.onNodeWithTag("gateway.console.connect").assertIsNotEnabled()
        composeRule.onNodeWithText("One gateway.\nComplete clarity.").assertIsDisplayed()
    }

    @Test
    fun validLookingCredentialsEnableTheFormWithoutNetwork() {
        composeRule.onNodeWithTag("gateway.console.address")
            .performTextInput("https://gateway.example.test")
        composeRule.onNodeWithTag("gateway.console.token")
            .performTextInput("synthetic-token-longer-than-thirty-two-characters")
        composeRule.onNodeWithTag("gateway.console.connect").assertIsEnabled()
        composeRule.onNodeWithText("Connect securely").assertIsDisplayed()
        // No button click: the test never contacts a real console.
        // GatewayUrlTest JVM tests separately verify rejection of HTTP URLs.
    }
}
