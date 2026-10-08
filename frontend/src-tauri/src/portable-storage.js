(() => {
  const storage = window.localStorage;
  const nativeSet = Storage.prototype.setItem;
  for (const [key, value] of Object.entries(window.__CLIPBOX_STORAGE__ || {})) {
    nativeSet.call(storage, key, value);
  }
  delete window.__CLIPBOX_STORAGE__;
  let writes = Promise.resolve();
  function persist() {
    const snapshot = Object.fromEntries(Array.from({ length: storage.length }, (_, index) => {
      const key = storage.key(index);
      return [key, storage.getItem(key)];
    }));
    writes = writes.catch(() => {}).then(() => window.__TAURI_INTERNALS__.invoke("save_portable_storage", { storage: snapshot }));
    writes.catch(error => console.error("Portable storage could not be saved", error));
  }
  for (const method of ["setItem", "removeItem", "clear"]) {
    const original = Storage.prototype[method];
    Storage.prototype[method] = function (...args) {
      const result = original.apply(this, args);
      if (this === storage) persist();
      return result;
    };
  }
})();
