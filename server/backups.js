import { readdir, open, lstat } from 'node:fs/promises';
import { constants } from 'node:fs';
import path from 'node:path';
import { validateDraft } from './caddy.js';

// Created by Gateway's snapshot writer. Reject arbitrary filenames and paths.
const NAME = /^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}[.][0-9]{3}Z-[a-f0-9]{10}[.]json$/;
const MAX_BYTES = 1024 * 1024;

export function validSnapshotName(name) {
  return typeof name === 'string' && NAME.test(name);
}

export async function listSnapshots(directory) {
  let entries;
  try { entries = await readdir(directory, { withFileTypes: true }); }
  catch (error) {
    if (error.code === 'ENOENT') return [];
    throw error;
  }
  const candidates = entries.filter(item => item.isFile() && validSnapshotName(item.name))
    .map(item => item.name).sort().reverse().slice(0, 100);
  const snapshots = [];
  for (const name of candidates) {
    try {
      const record = await lstat(path.join(directory, name));
      if (!record.isFile()) continue;
      snapshots.push({ name, bytes: record.size, modifiedAt: record.mtime.toISOString() });
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
  }
  return snapshots;
}

export async function readSnapshot(directory, name) {
  if (!validSnapshotName(name)) {
    throw Object.assign(new Error('Invalid snapshot name'), { status: 400 });
  }
  if (!constants.O_NOFOLLOW) {
    throw Object.assign(new Error('Safe snapshot staging is unsupported here'), { status: 501 });
  }
  let file;
  try {
    file = await open(path.join(directory, name), constants.O_RDONLY | constants.O_NOFOLLOW);
    const info = await file.stat();
    if (!info.isFile() || info.size > MAX_BYTES) {
      throw Object.assign(new Error('Snapshot cannot be staged (unsupported file or size)'), { status: 413 });
    }
    const content = await file.readFile({ encoding: 'utf8' });
    return { content: JSON.stringify(validateDraft(content), null, 2), name };
  } catch (error) {
    if (error.code === 'ENOENT') throw Object.assign(new Error('Snapshot not found'), { status: 404 });
    if (error.code === 'ELOOP') throw Object.assign(new Error('Snapshot is not a regular file'), { status: 400 });
    throw error;
  } finally { await file?.close(); }
}
