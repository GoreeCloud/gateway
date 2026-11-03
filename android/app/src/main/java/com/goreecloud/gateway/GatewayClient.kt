package com.goreecloud.gateway

import java.net.URI

data class GatewaySnapshot(
    val state: String,
    val engine: String,
    val servers: Int?,
    val routes: Int?,
    val hosts: List<String>,
    val hostCount: Int?,
    val checkedAt: String
)

fun validateConsoleUrl(raw: String): String {
    val value = raw.trim().trimEnd('/')
    val uri = try { URI(value) } catch (_: Exception) {
        throw IllegalArgumentException("Enter a valid HTTPS console address.")
    }
    require(
        uri.scheme.equals("https", ignoreCase = true) &&
            !uri.host.isNullOrBlank() &&
            uri.userInfo == null &&
            uri.rawQuery == null &&
            uri.rawFragment == null &&
            (uri.path.isNullOrEmpty() || uri.path == "/")
    ) { "Use a trusted HTTPS console address without a path, query or embedded credentials." }
    return value
}


class GatewayClient {
    suspend fun overview(console: String, accessToken: String): GatewaySnapshot =
        kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            val base = validateConsoleUrl(console)
            require(accessToken.length >= 32) { "Enter the complete access token." }
            val connection = java.net.URL("$base/api/v1/overview").openConnection()
                    as javax.net.ssl.HttpsURLConnection
            connection.requestMethod = "GET"
            connection.connectTimeout = 5000
            connection.readTimeout = 5000
            connection.instanceFollowRedirects = false
            connection.setRequestProperty("Authorization", "Bearer $accessToken")
            connection.setRequestProperty("Accept", "application/json")
            try {
                when (connection.responseCode) {
                    200 -> Unit
                    401 -> throw IllegalStateException("Access denied. Check your console token.")
                    else -> throw IllegalStateException("Console connection was unsuccessful.")
                }
                val payload = connection.inputStream.bufferedReader(Charsets.UTF_8).use { reader ->
                    val text = StringBuilder()
                    val block = CharArray(4096)
                    while (true) {
                        val count = reader.read(block)
                        if (count < 0) break
                        if (text.length + count > 65536) {
                            throw IllegalStateException("Console response exceeded the size limit.")
                        }
                        text.append(block, 0, count)
                    }
                    org.json.JSONObject(text.toString())
                }
                require(payload.optString("api_version") == "v1") { "Unsupported console API." }
                val state = payload.optString("state")
                require(state == "connected" || state == "unreachable") { "Unexpected response." }
                val hostsJson = payload.optJSONArray("hosts")
                    ?: throw IllegalStateException("Invalid host list.")
                val hosts = buildList {
                    for (index in 0 until minOf(100, hostsJson.length())) {
                        val name = hostsJson.optString(index)
                        if (name.isNotBlank() && name.length <= 253) add(name)
                    }
                }
                GatewaySnapshot(
                    state = state,
                    engine = payload.optString("engine", "Caddy-compatible"),
                    servers = if (payload.isNull("servers")) null else payload.optInt("servers"),
                    routes = if (payload.isNull("routes")) null else payload.optInt("routes"),
                    hosts = hosts,
                    hostCount = if (payload.isNull("host_count")) null else payload.optInt("host_count"),
                    checkedAt = payload.optString("checked_at")
                )
            } finally {
                connection.disconnect()
            }
        }
}
