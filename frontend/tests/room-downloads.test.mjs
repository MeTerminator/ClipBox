import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import ts from 'typescript';
import * as vue from 'vue';

// Exercise the room page's actual setup logic with native I/O replaced by a fixture.
function roomFixture(initialStorage = []) {
  let source = readFileSync(new URL('../src/views/Rooms.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
  const ast = ts.createSourceFile('Rooms.ts', source, ts.ScriptTarget.Latest, true);
  for (const node of [...ast.statements].reverse()) {
    if (ts.isImportDeclaration(node)) source = source.slice(0, node.getFullStart()) + source.slice(node.getEnd());
  }
  const storage = new Map(initialStorage);
  const paths = new Set();
  const calls = { downloads: 0, saveAs: 0, reveals: 0 };
  let cancel = false;
  const context = {
    ...vue, onMounted() {}, onBeforeUnmount() {},
    useRoute: () => ({ params: {} }), useRouter: () => ({ replace: async () => {} }),
    useI18n: () => ({ t: (key) => key }), toast: { success() {}, error() {}, info() {} },
    useFileUpload: () => ({}),
    localStorage: { getItem: (key) => storage.get(key) ?? null, setItem: (key, value) => storage.set(key, value) },
    backendOrigin: 'http://fixture', backendURL: (path) => `http://fixture${path}`, uploadBackendURL: async (path) => `http://fixture${path}`,
    isDesktopClient: () => true, setRoomFileDropHandler() {}, setRoomFileCodeHandler() {},
    setDesktopUploadTarget: async () => {}, setDesktopClipboardSharing: async () => {}, setDesktopSharedText: async () => {},
    desktopRoomFileExists: async (path) => paths.has(path),
    revealDesktopRoomFile: async (path) => { calls.reveals++; return paths.has(path); },
    saveDesktopRoomFile: async () => { calls.downloads++; await new Promise(setImmediate); paths.add('/downloads/file'); return '/downloads/file'; },
    saveDesktopRoomFileAs: async () => { calls.saveAs++; if (cancel) return null; paths.add('/chosen/file'); return '/chosen/file'; },
    setTimeout, clearTimeout, setInterval, clearInterval, console,
  };
  source += '\nglobalThis.api = {session, messages, autoDownload, downloadedRoomFiles, downloadedFileKey, roomFileActionTitle, saveRoomDownload, autoDownloadRoomFile, downloadFile};';
  vm.createContext(context);
  vm.runInContext(ts.transpile(source, { target: ts.ScriptTarget.ES2022 }), context);
  const api = context.api;
  api.session.value = { token: 'fixture', room: { id: '12345', current_member_id: 1, created_at: 'fixture', members: [] } };
  const message = (id, sha1 = 'content', name = 'file') => ({ id, kind: 'file', sender: { id: 2 }, file: { name, sha1, size: 4, download_url: `/api/rooms/12345/messages/${id}/file` } });
  return { api, calls, paths, storage, message, setCancel: (value) => { cancel = value; } };
}

test('concurrent repeated content downloads once and all copies become folder actions', async () => {
  const { api, calls, message } = roomFixture();
  const first = message(1), repeated = message(2, 'content', 'renamed');
  await Promise.all([api.autoDownloadRoomFile(first), api.autoDownloadRoomFile(repeated)]);
  assert.equal(calls.downloads, 1);
  assert.equal(api.roomFileActionTitle(first), 'rooms.openFileFolder');
  assert.equal(api.roomFileActionTitle(repeated), 'rooms.openFileFolder');
  api.session.value.room.id = '54321';
  await api.autoDownloadRoomFile(message(3));
  assert.equal(calls.downloads, 1);
  await api.downloadFile(repeated);
  assert.equal(calls.reveals, 1);
  assert.equal(calls.downloads, 1);
});

test('disabled automatic downloads use Save As, preserve cancellation, and remember chosen path', async () => {
  const { api, calls, message, storage, setCancel } = roomFixture();
  const file = message(1);
  api.autoDownload.value = false;
  await api.autoDownloadRoomFile(file);
  assert.equal(calls.downloads, 0);
  assert.equal(storage.get('desktopAutoDownload'), 'false');
  assert.equal(api.roomFileActionTitle(file), 'rooms.saveFileAs');
  setCancel(true);
  await api.downloadFile(file);
  assert.equal(api.downloadedRoomFiles.value[api.downloadedFileKey(file)], undefined);
  setCancel(false);
  await api.downloadFile(file);
  assert.equal(calls.saveAs, 2);
  assert.equal(api.downloadedRoomFiles.value[api.downloadedFileKey(file)], '/chosen/file');
  await api.downloadFile(file);
  assert.equal(calls.saveAs, 2);
  assert.equal(calls.reveals, 1);
});

test('deleted downloads are fetched again and re-enabling downloads catches up once', async () => {
  const { api, calls, paths, message } = roomFixture();
  const file = message(1);
  await api.autoDownloadRoomFile(file);
  paths.clear();
  await api.autoDownloadRoomFile(file);
  assert.equal(calls.downloads, 2);
  api.autoDownload.value = false;
  api.messages.value = [message(2, 'new-content'), message(3, 'new-content')];
  api.autoDownload.value = true;
  await api.saveRoomDownload(api.messages.value[0]);
  assert.equal(calls.downloads, 3);
});


test('saved content remains deduplicated after reopening the client', async () => {
  const first = roomFixture();
  await first.api.autoDownloadRoomFile(first.message(1));
  const reopened = roomFixture(first.storage);
  reopened.paths.add('/downloads/file');
  await reopened.api.autoDownloadRoomFile(reopened.message(9, 'content', 'renamed'));
  assert.equal(reopened.calls.downloads, 0);
  assert.equal(reopened.api.roomFileActionTitle(reopened.message(9)), 'rooms.openFileFolder');
});
