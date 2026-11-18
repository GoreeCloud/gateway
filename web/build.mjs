import { build } from 'esbuild'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'

await rm('dist', { recursive: true, force: true })
await mkdir('dist/assets', { recursive: true })
await build({
  entryPoints: ['src/main.tsx'],
  outdir: 'dist/assets',
  entryNames: 'app',
  bundle: true,
  minify: true,
  format: 'esm',
  target: ['es2022'],
  jsx: 'automatic',
  sourcemap: false,
  logLevel: 'info'
})
const html = await readFile('index.html', 'utf8')
const updated = html.replace(
  '<script type="module" src="/src/main.tsx"></script>',
  '<link rel="stylesheet" href="/assets/app.css" /><script type="module" src="/assets/app.js"></script>'
)
if (updated === html) throw new Error('Web entrypoint not found')
await writeFile('dist/index.html', updated)
