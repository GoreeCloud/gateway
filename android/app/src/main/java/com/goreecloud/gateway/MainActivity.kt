package com.goreecloud.gateway

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

private val deepCanvas = Color(0xFF0C1223)
private val deepSurface = Color(0xFF172039)
private val deepTonal = Color(0xFF223452)
private val deepAccent = Color(0xFFB7C4FF)

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { GatewayApp() }
    }
}

@Composable
private fun GatewayApp() {
    val colors = if (isSystemInDarkTheme()) {
        darkColorScheme(
            primary = deepAccent,
            onPrimary = Color(0xFF16224A),
            background = deepCanvas,
            surface = deepSurface,
            surfaceVariant = deepTonal,
            onSurface = Color(0xFFF6F8FF),
            onSurfaceVariant = Color(0xFF9AAAC8)
        )
    } else {
        lightColorScheme(
            primary = Color(0xFF5269C0),
            background = Color(0xFFF0F4FB),
            surface = Color.White,
            surfaceVariant = Color(0xFFE5EBFA)
        )
    }
    val client = remember { GatewayClient() }
    val scope = rememberCoroutineScope()
    var url by remember { mutableStateOf("") }
    var inputToken by remember { mutableStateOf("") }
    var liveToken by remember { mutableStateOf("") }
    var activeUrl by remember { mutableStateOf("") }
    var snapshot by remember { mutableStateOf<GatewaySnapshot?>(null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var tab by remember { mutableIntStateOf(0) }
    fun connect(address: String, key: String, first: Boolean) {
        if (busy) return
        scope.launch {
            busy = true
            error = null
            try {
                val result = client.overview(address, key)
                snapshot = result
                if (first) {
                    activeUrl = validateConsoleUrl(address)
                    liveToken = key
                    inputToken = ""
                    tab = 0
                }
            } catch (exception: Exception) {
                error = when (exception) {
                    is IllegalStateException, is IllegalArgumentException -> exception.message
                    else -> "Cannot reach the private console. Check HTTPS and network access."
                }
            } finally {
                busy = false
            }
        }
    }
    fun lock() {
        liveToken = ""
        inputToken = ""
        snapshot = null
        error = null
        tab = 0
    }
    MaterialTheme(colorScheme = colors) {
        Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            if (snapshot == null) {
                ConnectPanel(
                    url, inputToken, busy, error,
                    { url = it }, { inputToken = it },
                    { connect(url, inputToken, true) }
                )
            } else {
                Scaffold(
                    containerColor = MaterialTheme.colorScheme.background,
                    topBar = { BrandHeader() },
                    bottomBar = {
                        NavigationBar(containerColor = MaterialTheme.colorScheme.surface) {
                            val labels = listOf("Overview", "Sites", "Security", "Settings")
                            val marks = listOf("◈", "◎", "⬡", "⚙")
                            labels.forEachIndexed { index, name ->
                                NavigationBarItem(
                                    selected = tab == index,
                                    onClick = { tab = index },
                                    icon = { Text(marks[index], fontSize = 20.sp) },
                                    label = { Text(name, fontSize = 11.sp) }
                                )
                            }
                        }
                    }
                ) { padding ->
                    Column(
                        Modifier.padding(padding).fillMaxSize()
                            .verticalScroll(rememberScrollState())
                            .padding(horizontal = 22.dp, vertical = 24.dp),
                        verticalArrangement = Arrangement.spacedBy(17.dp)
                    ) {
                        val data = snapshot!!
                        when (tab) {
                            0 -> {
                                PageTitle("Overview", "Your private edge at a glance.")
                                EngineCard(data)
                                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                    MetricCard("Servers", data.servers?.toString() ?: "—", Modifier.weight(1f))
                                    MetricCard("Routes", data.routes?.toString() ?: "—", Modifier.weight(1f))
                                    MetricCard("Hosts", if (data.state == "connected") data.hosts.size.toString() else "—", Modifier.weight(1f))
                                }
                                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                    Text("Active hostnames", fontSize = 20.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                                    OutlinedButton(onClick = { connect(activeUrl, liveToken, false) }, enabled = !busy) {
                                        Text(if (busy) "Checking" else "Refresh")
                                    }
                                }
                                if (error != null) Text(error ?: "", color = MaterialTheme.colorScheme.error, fontSize = 13.sp)
                                if (data.hosts.isEmpty()) HintCard("No hostname matchers are currently available.")
                                data.hosts.take(8).forEach { HostCard(it) }
                                if (data.hosts.size > 8) HintCard("Additional sites are listed under Sites.")
                                Text("Observed " + data.checkedAt, color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 11.sp)
                            }
                            1 -> {
                                PageTitle("Sites & routes", "Hostnames in the active HTTP configuration.")
                                if (data.state != "connected") HintCard("Engine offline; routing data is unavailable.")
                                else if (data.hosts.isEmpty()) HintCard("No host matchers found in top-level routes.")
                                else data.hosts.forEach { HostCard(it) }
                            }
                            2 -> {
                                PageTitle("Security", "Controls protecting this console.")
                                InfoCard("TOKEN PROTECTED", "Authenticated API", "Every overview request requires your private access token.")
                                InfoCard("HTTPS ONLY", "Native secure transport", "The Android app refuses cleartext and redirects.")
                                InfoCard("READ ONLY", "Protected configuration", "Clients never receive raw Caddy configuration or credentials.")
                                InfoCard("NOT AVAILABLE", "Administration actions", "Route edits and restarts are intentionally not implemented.")
                                HintCard("This screen describes client safeguards, not a VPS security certification.")
                            }
                            else -> {
                                PageTitle("Settings", "Control your private session.")
                                InfoCard("CONNECTED", "Console URL", activeUrl)
                                InfoCard("IN MEMORY", "Access token", "Credentials are not persisted to device storage.")
                                OutlinedButton(onClick = { lock() }, modifier = Modifier.fillMaxWidth()) { Text("Lock and clear access") }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun BrandHeader() {
    Row(Modifier.fillMaxWidth().padding(20.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        BrandMark()
        Column {
            Text("GoreeCloud", fontWeight = FontWeight.Bold, fontSize = 18.sp)
            Text("GATEWAY / CONTROL", color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 10.sp, letterSpacing = 1.2.sp)
        }
    }
}

@Composable
private fun BrandMark() {
    Box(
        Modifier.size(50.dp).clip(RoundedCornerShape(17.dp))
            .background(Brush.linearGradient(listOf(Color(0xFFD2DBFF), Color(0xFF7189F6)))),
        contentAlignment = Alignment.Center
    ) { Text("◇", color = Color(0xFF1B2A55), fontSize = 34.sp, fontWeight = FontWeight.Bold) }
}

@Composable
private fun ConnectPanel(
    url: String,
    token: String,
    busy: Boolean,
    error: String?,
    onUrl: (String) -> Unit,
    onToken: (String) -> Unit,
    onConnect: () -> Unit
) {
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(23.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp)
    ) {
        BrandHeader()
        Spacer(Modifier.height(16.dp))
        Text("PRIVATE INFRASTRUCTURE", color = MaterialTheme.colorScheme.primary, fontWeight = FontWeight.Bold, letterSpacing = 2.sp, fontSize = 11.sp)
        Text("One gateway.\nComplete clarity.", fontWeight = FontWeight.Bold, fontSize = 39.sp, lineHeight = 43.sp)
        Text("Your self-hosted edge, seen clearly. No cloud account. No direct admin API access.", color = MaterialTheme.colorScheme.onSurfaceVariant, lineHeight = 23.sp)
        Card(
            shape = RoundedCornerShape(28.dp),
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.surfaceVariant)
        ) {
            Column(Modifier.padding(24.dp), verticalArrangement = Arrangement.spacedBy(15.dp)) {
                BrandMark()
                Text("Connect securely", fontWeight = FontWeight.Bold, fontSize = 25.sp)
                Text("Enter your HTTPS console address and access token. The token stays only in memory.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                OutlinedTextField(value = url, onValueChange = onUrl, label = { Text("HTTPS console URL") }, placeholder = { Text("https://gateway.example.org") }, singleLine = true, modifier = Modifier.fillMaxWidth())
                OutlinedTextField(value = token, onValueChange = onToken, label = { Text("Access token") }, visualTransformation = PasswordVisualTransformation(), singleLine = true, modifier = Modifier.fillMaxWidth())
                if (error != null) Text(error, color = MaterialTheme.colorScheme.error, fontSize = 13.sp)
                Button(onClick = onConnect, enabled = !busy && url.isNotBlank() && token.length >= 32, modifier = Modifier.fillMaxWidth().height(54.dp), shape = RoundedCornerShape(16.dp)) {
                    Text(if (busy) "Connecting…" else "Open Gateway  ↗", fontWeight = FontWeight.Bold)
                }
            }
        }
        Text("Use a trusted private HTTPS endpoint. Editing and deployments are not available in this development build.", color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
    }
}

@Composable
private fun EngineCard(data: GatewaySnapshot) {
    val online = data.state == "connected"
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(27.dp))
            .background(Brush.horizontalGradient(listOf(Color(0xFF374A85), Color(0xFF19465D))))
            .padding(26.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text("CURRENT CONNECTION", color = Color(0xFFD4DFFF), fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp)
        Text(if (online) "Your edge is responding." else "Engine not reachable", color = Color.White, fontSize = 25.sp, lineHeight = 30.sp, fontWeight = FontWeight.Bold)
        Text(if (online) "Live local configuration summary." else "Console reachable; engine metrics unknown.", color = Color(0xFFD7E3F6))
        Text(if (online) "● ENGINE ONLINE" else "● ENGINE OFFLINE", color = if (online) Color(0xFF92F2C7) else Color(0xFFFFC3BB), fontSize = 12.sp, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun MetricCard(title: String, value: String, modifier: Modifier) {
    Card(modifier, shape = RoundedCornerShape(19.dp), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface), border = BorderStroke(1.dp, MaterialTheme.colorScheme.surfaceVariant)) {
        Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(title, fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text(value, fontSize = 30.sp, fontWeight = FontWeight.Bold)
        }
    }
}

@Composable
private fun HostCard(host: String) {
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface), shape = RoundedCornerShape(18.dp), border = BorderStroke(1.dp, MaterialTheme.colorScheme.surfaceVariant)) {
        Row(Modifier.fillMaxWidth().padding(17.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(Modifier.size(43.dp).clip(RoundedCornerShape(13.dp)).background(MaterialTheme.colorScheme.surfaceVariant), contentAlignment = Alignment.Center) {
                Text("◎", color = MaterialTheme.colorScheme.primary, fontSize = 25.sp)
            }
            Column(Modifier.weight(1f)) {
                Text(host, fontWeight = FontWeight.SemiBold, overflow = TextOverflow.Ellipsis, maxLines = 1)
                Text("Configured hostname", fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@Composable
private fun InfoCard(label: String, title: String, detail: String) {
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface), shape = RoundedCornerShape(20.dp), border = BorderStroke(1.dp, MaterialTheme.colorScheme.surfaceVariant)) {
        Column(Modifier.fillMaxWidth().padding(19.dp), verticalArrangement = Arrangement.spacedBy(7.dp)) {
            Text(label, color = MaterialTheme.colorScheme.primary, fontSize = 10.sp, letterSpacing = 1.sp, fontWeight = FontWeight.Bold)
            Text(title, fontSize = 17.sp, fontWeight = FontWeight.Bold)
            Text(detail, fontSize = 13.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun HintCard(message: String) {
    Surface(Modifier.fillMaxWidth(), color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f), shape = RoundedCornerShape(18.dp)) {
        Text(message, Modifier.padding(20.dp), color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 13.sp)
    }
}

@Composable
private fun PageTitle(title: String, description: String) {
    Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
        Text("GATEWAY / CONTROL CENTER", color = MaterialTheme.colorScheme.primary, fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp)
        Text(title, fontSize = 33.sp, fontWeight = FontWeight.Bold)
        Text(description, color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 14.sp)
    }
}
