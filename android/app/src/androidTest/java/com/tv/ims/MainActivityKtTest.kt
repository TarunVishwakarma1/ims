package com.tv.ims

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.tv.ims.ui.theme.ImsTheme
import org.junit.Rule
import org.junit.Test

class MainActivityKtTest {

    @get:Rule
    val composeTestRule = createComposeRule()

    @Test
    fun testNavigationTabs() {
        composeTestRule.setContent {
            ImsTheme {
                ImsApp()
            }
        }

        // Verify Home is initially selected and displaying its content
        composeTestRule.onNodeWithText("Home Content").assertExists()

        // Click Favorites tab
        composeTestRule.onNodeWithText("Favorites").performClick()
        // Verify Favorites content is displayed
        composeTestRule.onNodeWithText("Favorites Content").assertExists()

        // Click Profile tab
        composeTestRule.onNodeWithText("Profile").performClick()
        // Verify Profile content is displayed
        composeTestRule.onNodeWithText("Profile Content").assertExists()
    }
}
