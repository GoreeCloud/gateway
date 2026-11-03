(() => {
"use strict";
let token = "";
let data = null;
const el = (id) => document.getElementById(id);
const pages = {
 overview: ["Overview", "A clear view of your private edge."],
 sites: ["Sites & routes", "Explore configured hostnames and routing matchers."],
 security: ["Security", "Understand your console's protective boundaries."],
 settings: ["Settings", "Keep your session and appearance under control."]
};

function setBadge(text, state) {
 const badge = el("connectionBadge");
 badge.textContent = "● " + text;
 badge.dataset.state = state;
}
function navigate(page) {
 if (!pages[page]) return;
 el("pageTitle").textContent = pages[page][0];
 el("pageSubtitle").textContent = pages[page][1];
 el("breadcrumbTitle").textContent = pages[page][0];
 for (const view of document.querySelectorAll(".view")) view.hidden = view.id !== page + "View";
 for (const button of document.querySelectorAll("[data-tab]")) {
  const selected = button.dataset.tab === page;
  button.classList.toggle("active", selected);
  if (selected) button.setAttribute("aria-current", "page");
  else button.removeAttribute("aria-current");
 }
 if (page === "sites") renderHosts();
}
function hostRow(name) {
 const row = document.createElement("div");
 row.className = "host-row";
 const emblem = document.createElement("span");
 emblem.className = "host-emblem";
 emblem.textContent = "◎";
 const title = document.createElement("span");
 title.className = "host-name";
 title.textContent = name;
 const tag = document.createElement("span");
 tag.className = "host-tag";
 tag.textContent = "Configured";
 row.append(emblem, title, tag);
 return row;
}
function empty(text) {
 const node = document.createElement("p");
 node.className = "empty";
 node.textContent = text;
 return node;
}
function renderHosts() {
 const full = el("siteHosts");
 const preview = el("overviewHosts");
 full.replaceChildren();
 preview.replaceChildren();
 const names = data && data.state === "connected" ? data.hosts : [];
 const search = el("hostSearch").value.trim().toLowerCase();
 for (const host of names.filter((name) => name.toLowerCase().includes(search))) full.append(hostRow(host));
 for (const host of names.slice(0, 4)) preview.append(hostRow(host));
 if (!full.children.length) full.append(empty(names.length ? "No matching hostnames." : "No hostname data is available."));
 if (!preview.children.length) preview.append(empty(data && data.state === "connected" ? "No host matchers found in top-level routes." : "No live hostname data is available."));
}
function render() {
 if (!data) return;
 const connected = data.state === "connected";
 el("heroTitle").textContent = connected ? "Your edge is responding." : "Engine connection unavailable";
 el("heroDescription").textContent = connected ? "Live summary of the local Caddy-compatible configuration." : "The console is available, but its local engine is not reachable. Metrics are intentionally unknown.";
 el("serverCount").textContent = connected ? String(data.servers) : "—";
 el("routeCount").textContent = connected ? String(data.routes) : "—";
 el("hostCount").textContent = connected ? String(data.hosts.length) : "—";
 el("lastChecked").textContent = data.checked_at ? "Observed " + new Date(data.checked_at).toLocaleString() : "No observation yet";
 setBadge(connected ? "Engine online" : "Engine offline", connected ? "online" : "offline");
 renderHosts();
}
async function requestOverview(key) {
 const response = await fetch("/api/v1/overview", {
  method: "GET",
  headers: {Authorization: "Bearer " + key},
  cache: "no-store",
  credentials: "omit",
  redirect: "error"
 });
 if (response.status === 401) throw new Error("unauthorized");
 if (!response.ok) throw new Error("unavailable");
 const payload = await response.json();
 if (payload.api_version !== "v1" || !Array.isArray(payload.hosts) || !["connected", "unreachable"].includes(payload.state)) throw new Error("invalid response");
 return payload;
}
async function refresh() {
 if (!token) return;
 el("refreshButton").disabled = true;
 el("pageRefresh").disabled = true;
 try {
  data = await requestOverview(token);
  render();
 } catch (error) {
  if (error.message === "unauthorized") {
   lock();
   showError("Your session is no longer authorized.");
  } else {
   data = {state: "unreachable", hosts: []};
   render();
   setBadge("Connection lost", "offline");
  }
 } finally {
  el("refreshButton").disabled = false;
  el("pageRefresh").disabled = false;
 }
}
function showError(message) {
 el("loginError").textContent = message;
 el("loginError").hidden = false;
}
function lock() {
 token = "";
 data = null;
 el("tokenInput").value = "";
 el("loginPanel").hidden = false;
 el("dashboardPanel").hidden = true;
 setBadge("Not connected", "offline");
 el("tokenInput").focus();
}
el("loginForm").addEventListener("submit", async (event) => {
 event.preventDefault();
 const candidate = el("tokenInput").value;
 el("loginError").hidden = true;
 if (candidate.length < 32) { showError("Enter the complete 32+ character console token."); return; }
 el("connectButton").disabled = true;
 try {
  const result = await requestOverview(candidate);
  token = candidate;
  data = result;
  el("tokenInput").value = "";
  el("loginPanel").hidden = true;
  el("dashboardPanel").hidden = false;
  render();
  navigate("overview");
 } catch (error) {
  showError(error.message === "unauthorized" ? "Access denied. Check your token." : "Unable to reach a compatible console.");
 } finally { el("connectButton").disabled = false; }
});
for (const node of document.querySelectorAll("[data-tab]")) node.addEventListener("click", () => navigate(node.dataset.tab));
for (const id of ["railLock", "settingLock"]) el(id).addEventListener("click", lock);
for (const id of ["refreshButton", "pageRefresh"]) el(id).addEventListener("click", refresh);
el("openSites").addEventListener("click", () => navigate("sites"));
el("hostSearch").addEventListener("input", renderHosts);
function switchTheme() {
 document.documentElement.classList.toggle("light");
 try { localStorage.setItem("gc-gateway-theme", document.documentElement.classList.contains("light") ? "light" : "dark"); } catch {}
}
for (const id of ["themeButton", "settingTheme"]) el(id).addEventListener("click", switchTheme);
try { if (localStorage.getItem("gc-gateway-theme") === "light") document.documentElement.classList.add("light"); } catch {}
navigate("overview");
})();