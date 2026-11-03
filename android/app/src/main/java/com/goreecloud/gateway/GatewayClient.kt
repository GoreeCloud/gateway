package com.goreecloud.gateway

import java.net.URI

data class GatewaySnapshot(
    val state: String,
    val engine: String,
    val servers: Int?,
    val routes: Int?,
    val hosts: List<String>,
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
