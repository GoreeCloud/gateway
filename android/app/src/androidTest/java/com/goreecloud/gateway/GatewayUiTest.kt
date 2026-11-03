package com.goreecloud.gateway

import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
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
    fun rejectsInsecureEndpointBeforeMakingNetworkRequest() {
        composeRule.onNodeWithTag("gateway.console.address")
            .performTextInput("http://test.invalid")
        composeRule.onNodeWithTag("gateway.console.token")
            .performTextInput("untrusted-test-token-length-40-characters!")
        composeRule.onNodeWithTag("gateway.console.connect")
            .assertIsEnabled()
            .performClick()

        composeRule.waitUntil(timeoutMillis = 10000) {
            composeRule.onAllNodesWithText(
                "Use a trusted HTTPS console address without a path, query or embedded credentials."
            ).fetchSemanticsNodes().isNotEmpty()
        }
        composeRule.onNodeWithText("Connect securely").assertIsDisplayed()
    }
}
