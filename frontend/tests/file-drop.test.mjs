import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import ts from 'typescript';
import * as vue from 'vue';

function setup(path, context, exports) {
  let source = readFileSync(new URL(path, import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
  const ast = ts.createSourceFile('setup.ts', source, ts.ScriptTarget.Latest, true);
  for (const node of [...ast.statements].reverse()) {
    if (ts.isImportDeclaration(node)) source = source.slice(0, node.getFullStart()) + source.slice(node.getEnd());
  }
  source += `\nglobalThis.api = {${exports}};`;
  vm.createContext(context);
  vm.runInContext(ts.transpile(source, { target: ts.ScriptTarget.ES2022 }), context);
  return context.api;
}

function fixture(page = 'create') {
  const route = { name: page, query: { tab: 'file', redirect: 'https://example.com/callback' } };
  const uploads = [], navigations = [], roomFiles = [];
  const shared = {
    URL,
    ...vue, onMounted() {}, onBeforeUnmount() {},
    pendingShareFile: vue.shallowRef(null),
    roomFileDropHandler: vue.shallowRef(null), roomFileCodeHandler: vue.shallowRef(null),
    useI18n: () => ({ t: (key) => key }),
    useRoute: () => route,
    useRouter: () => ({ currentRoute: vue.ref(route), push: async (target) => { navigations.push(target); } }),
    useFileUpload: () => ({
      uploadStage: vue.ref(null), uploadProgress: vue.ref(0), resumedChunks: vue.ref(0),
      resetUploadProgress() {},
      uploadFile: async (file, options) => { uploads.push({ file, ...options }); return { code: '12345' }; },
    }),
    localStorage: { getItem: () => null, setItem() {} },
    toast: { success() {}, error() {} },
  };
  const app = setup('../src/App.vue', { ...shared }, 'handleDrop');
  const mountClip = () => setup('../src/views/Clip.vue', { ...shared }, 'selectedFile, activeTab, formData, loading, submitClip, detailModalOpen, pendingRedirect');
  const drop = (file) => app.handleDrop({ preventDefault() {}, dataTransfer: { types: ['Files'], files: [file] } });
  return { shared, uploads, navigations, roomFiles, mountClip, drop };
}

test('drop selects a file without uploading, then submission uses edited access options', async () => {
  const f = fixture();
  const clip = f.mountClip();
  clip.activeTab.value = 'text';
  clip.formData.value.count = 7;
  clip.formData.value.expire = 2;
  clip.formData.value.expireUnit = '3600';
  const file = new File(['hello'], 'hello.txt');
  await f.drop(file);
  await vue.nextTick();
  assert.equal(f.uploads.length, 0);
  assert.equal(f.navigations.length, 0);
  assert.equal(clip.selectedFile.value, file);
  assert.equal(clip.activeTab.value, 'file');
  assert.equal(clip.detailModalOpen.value, false);
  await clip.submitClip();
  assert.equal(f.uploads.length, 1);
  assert.equal(f.uploads[0].file, file);
  assert.equal(f.uploads[0].count, 7);
  assert.equal(f.uploads[0].expire, 7200);
  assert.equal(clip.detailModalOpen.value, true);
  assert.equal(new URL(clip.pendingRedirect.value).searchParams.get('filecode'), '12345');
});

test('drop on home carries the file to the upload form after navigation', async () => {
  const f = fixture('home');
  const file = new File(['hello'], 'hello.txt');
  await f.drop(file);
  assert.equal(f.uploads.length, 0);
  assert.equal(f.navigations.length, 1);
  assert.equal(f.navigations[0].name, 'create');
  const clip = f.mountClip();
  assert.equal(clip.selectedFile.value, file);
  assert.equal(f.shared.pendingShareFile.value, null);
});

test('room drops retain immediate sending', async () => {
  const f = fixture('rooms');
  f.shared.roomFileDropHandler.value = async (file) => { f.roomFiles.push(file); };
  const file = new File(['hello'], 'hello.txt');
  await f.drop(file);
  assert.equal(f.roomFiles[0], file);
  assert.equal(f.shared.pendingShareFile.value, null);
  assert.equal(f.navigations.length, 0);
});

test('dropping another file during submission does not replace the uploading file', async () => {
  const f = fixture();
  const clip = f.mountClip();
  const first = new File(['first'], 'first.txt');
  await f.drop(first);
  await vue.nextTick();
  clip.loading.value = true;
  await f.drop(new File(['second'], 'second.txt'));
  await vue.nextTick();
  assert.equal(clip.selectedFile.value, first);
  assert.equal(f.shared.pendingShareFile.value, null);
});
