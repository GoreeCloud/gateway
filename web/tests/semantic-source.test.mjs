import test from "node:test";
import assert from "node:assert/strict";
import {createHash} from "node:crypto";
import {readFileSync} from "node:fs";
const root=new URL("../",import.meta.url);
const read=p=>readFileSync(new URL(p,root),"utf8");
const provenance=JSON.parse(read("src/vendor/gc-ui/provenance.json"));
test("Gateway imports immutable first-party UI alpha tokens with MIT attribution",()=>{
 assert.equal(provenance.upstream,"https://github.com/GoreeCloud/gc-ui");
 assert.equal(provenance.sourceCommit,"be8b819748d1ef0f6b2d3e786f4b22864cb3d07d");
 for(const [file,sha] of Object.entries(provenance.sha256)){
  assert.match(sha,/^[0-9a-f]{64}$/);
  assert.equal(createHash("sha256").update(read("src/vendor/gc-ui/"+file)).digest("hex"),sha,file);
 }
 assert.match(read("src/vendor/gc-ui/LICENSE"),/MIT License/);
 assert.doesNotMatch(read("src/vendor/gc-ui/tokens.css"),/@import\s+(?:url\()?\s*["']?https?:\/\//i);
});
test("dark Iris default and light toggle share semantic theme scopes",()=>{
 const html=read("index.html"),main=read("src/main.tsx"),styles=read("src/styles.css");
 assert.match(html,/<html lang="en" data-gc-theme="dark" data-theme="dark">/);
 assert.ok(main.indexOf("import './vendor/gc-ui/tokens.css'")>=0);
 assert.ok(main.indexOf("import './styles.css'")>main.indexOf("import './vendor/gc-ui/tokens.css'"));
 assert.match(main,/document\.documentElement\.dataset\.theme = theme; document\.documentElement\.dataset\.gcTheme = theme/);
 for(const [local,source] of Object.entries({
  canvas:"gc-canvas",surface:"gc-surface",tonal:"gc-surface-tonal",text:"gc-text",
  muted:"gc-text-soft",outline:"gc-outline-soft",brand:"gc-brand",onbrand:"gc-on-brand",
  "brand-container":"gc-brand-container",positive:"gc-positive",warning:"gc-warning",
  negative:"gc-negative",radius:"gc-radius-md"
 })){
  assert.ok(styles.includes("--"+local+":var(--"+source+")"),"missing role "+local);
 }
 assert.doesNotMatch(styles,/--(?:canvas|surface|brand|onbrand):\s*#[0-9A-Fa-f]{3,8}/);
 assert.match(styles,/\.hero\{/);
 assert.match(styles,/\.route-table-row\{/);
});
