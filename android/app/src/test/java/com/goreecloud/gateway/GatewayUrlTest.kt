package com.goreecloud.gateway

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class GatewayUrlTest {
    @Test
    fun acceptsHttpsEndpoint() {
        assertEquals("https://gateway.example.test", validateConsoleUrl(" https://gateway.example.test/ "))
    }

    @Test
    fun rejectsPlainHttpAndCredentialInUrl() {
        for (value in listOf("http://gateway.example.test", "https://user:pass@gateway.example.test", "https://gateway.example.test/path")) {
            assertThrows(IllegalArgumentException::class.java) { validateConsoleUrl(value) }
        }
    }
}
