import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { listSnapshots, readSnapshot, validSnapshotName } from '../server/backups.js';

const valid = '2026-10-27T20-00-00.000Z-0123456789.json';

test('private inventory and staging require Gateway snapshot filenames', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'gateway-snapshot-'));
  try {
    await writeFile(join(dir, valid), JSON.stringify({ apps: { http: { servers: {} } } }), { mode: 0o600 });
    await writeFile(join(dir, 'unrelated.txt'), 'not a snapshot');
    const inventory = await listSnapshots(dir);
    assert.equal(inventory.length, 1);
    assert.equal(inventory[0].name, valid);
    assert.ok(inventory[0].bytes > 0);
    assert.ok(inventory[0].modifiedAt);
    assert.equal(validSnapshotName(valid), true);
    assert.equal(validSnapshotName('../file.json'), false);
    await assert.rejects(readSnapshot(dir, '../unrelated.txt'), /Invalid snapshot name/);
    const staged = await readSnapshot(dir, valid);
    assert.deepEqual(JSON.parse(staged.content), { apps: { http: { servers: {} } } });
  } finally { await rm(dir, { recursive: true, force: true }); }
});

test('snapshot loader rejects symbolic links and oversized files', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'gateway-snapshot-'));
  const external = join(dir, 'external.txt');
  const other = '2026-10-27T20-01-00.000Z-aabbccddee.json';
  try {
    await writeFile(external, '{}');
    await symlink(external, join(dir, valid));
    assert.deepEqual(await listSnapshots(dir), []);
    await assert.rejects(readSnapshot(dir, valid), /Snapshot is not a regular file/);
    await writeFile(join(dir, other), 'x'.repeat(1024 * 1024 + 1));
    await assert.rejects(readSnapshot(dir, other), /Snapshot cannot be staged/);
  } finally { await rm(dir, { recursive: true, force: true }); }
});

test('missing snapshot directory has an empty inventory', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'gateway-snapshot-'));
  try { assert.deepEqual(await listSnapshots(join(dir, 'missing')), []); }
  finally { await rm(dir, { recursive: true, force: true }); }
});
