import { mkdir, copyFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
await mkdir(path.join(root, 'dist'), { recursive: true });
for (const file of ['index.html', 'styles.css', 'app.js', 'model.js', 'mark.svg']) {
  await copyFile(path.join(root, 'src', file), path.join(root, 'dist', file));
}
await writeFile(path.join(root, 'dist', 'build.json'), JSON.stringify({ product: 'GoreeCloud Gateway', version: '0.1.0' }, null, 2));
console.log('Built standalone web assets in dist/ (no external CDNs or build dependencies)');
