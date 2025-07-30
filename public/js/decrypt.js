async function decryptZip() {
    const zipFile = document.getElementById('zipInput').files[0];
    if (!zipFile) return alert("请选择加密zip");

    const reader = new zip.ZipReader(new zip.BlobReader(zipFile));
    const entries = await reader.getEntries();

    const keyEntry = entries.find(e => e.filename === 'key.bin');
    const fileEntry = entries.find(e => e.filename === 'encrypted.file');

    if (!keyEntry || !fileEntry) return alert("ZIP中缺少必要文件");

    const keyBin = new Uint8Array(await (await keyEntry.getData(new zip.BlobWriter())).arrayBuffer());
    const encryptedData = new Uint8Array(await (await fileEntry.getData(new zip.BlobWriter())).arrayBuffer());

    const keyRaw = keyBin.slice(0, 16);
    const iv = keyBin.slice(16, 32);
    const encryptedName = keyBin.slice(32);

    const key = await crypto.subtle.importKey("raw", keyRaw, { name: "AES-CTR" }, false, ["decrypt"]);

    const decryptedNameBuf = await crypto.subtle.decrypt({ name: "AES-CTR", counter: iv, length: 64 }, key, encryptedName);
    const fileName = new TextDecoder().decode(decryptedNameBuf);

    const decryptedData = await crypto.subtle.decrypt({ name: "AES-CTR", counter: iv, length: 64 }, key, encryptedData);

    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([decryptedData]));
    a.download = fileName;
    a.click();

    await reader.close();
}