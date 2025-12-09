// 语言包定义
const translationsClips = {
    'zh': {
        // Tab Names
        'tab.text': '文本',
        'tab.link': '短链接',
        'tab.file': '文件',
        'tab.history': '历史',

        // Units
        'unit.day': '天',
        'unit.week': '周',
        'unit.month': '月',

        // File Sizes
        'size.bytes': '字节',
        'size.kb': 'KB',
        'size.mb': 'MB',
        'size.gb': 'GB',

        // History
        'history.file': '(文件)',
        'history.link': '(短链接)',
        'history.text': '(文本)',
        'history.clearConfirm': '确定要清空历史吗？',
        'history.cleared': '🧹 已清空历史记录',
        'history.clearBtn': '清空历史',

        // Progress/Info
        'info.calculating': '计算中',
        'info.hashing': '计算哈希',
        'info.prepare': '准备中...',
        'info.uploading': '上传中...',
        'info.selectFile': '❌ 请选择文件',
        'info.uploadSuccess': '✅ 上传成功',
        'info.instantUpload': ' (秒传)',

        // Errors/Messages
        'error.sizeLimit': '❌ 错误: 文本内容过大，已超过服务器限制',
        'error.linkSizeLimit': '❌ 错误: 链接内容过大，已超过服务器限制',
        'error.parseResponse': '❌ 错误: 无法解析服务器响应',
        'error.requestFailed': '请求失败',
        'error.uploadSizeLimit': '❌ 上传失败: 文件过大，已超过服务器限制',
        'error.parseUploadResponse': '❌ 解析响应失败',
        'error.uploadFailed': '❌ 上传失败:',
        'error.network': '❌ 网络错误',
        'error.unknown': '❌ 错误:',

        // Result/Copy
        'result.success': '✅ 创建成功：',
        'copy.success': '已复制',
        'copy.failed': '复制失败',
        'copy.hash': '复制',
        'copy.code': '复制提取码',

        // Theme
        'theme.light': '切换到日间模式',
        'theme.dark': '切换到夜间模式',
    },
    'en': {
        // Tab Names
        'tab.text': 'Text',
        'tab.link': 'Short Link',
        'tab.file': 'File',
        'tab.history': 'History',

        // Units
        'unit.day': 'Day',
        'unit.week': 'Week',
        'unit.month': 'Month',

        // File Sizes
        'size.bytes': 'Bytes',
        'size.kb': 'KB',
        'size.mb': 'MB',
        'size.gb': 'GB',

        // History
        'history.file': '(File)',
        'history.link': '(Short Link)',
        'history.text': '(Text)',
        'history.clearConfirm': 'Are you sure you want to clear the history?',
        'history.cleared': '🧹 History Cleared',
        'history.clearBtn': 'Clear History',

        // Progress/Info
        'info.calculating': 'Calculating',
        'info.hashing': 'Calculating Hash',
        'info.prepare': 'Preparing...',
        'info.uploading': 'Uploading...',
        'info.selectFile': '❌ Please select a file',
        'info.uploadSuccess': '✅ Upload Success',
        'info.instantUpload': ' (Instant)',

        // Errors/Messages
        'error.sizeLimit': '❌ Error: Text content is too large, exceeding server limit',
        'error.linkSizeLimit': '❌ Error: Link content is too large, exceeding server limit',
        'error.parseResponse': '❌ Error: Failed to parse server response',
        'error.requestFailed': 'Request failed',
        'error.uploadSizeLimit': '❌ Upload Failed: File too large, exceeding server limit',
        'error.parseUploadResponse': '❌ Failed to parse response',
        'error.uploadFailed': '❌ Upload Failed:',
        'error.network': '❌ Network Error',
        'error.unknown': '❌ Error:',

        // Result/Copy
        'result.success': '✅ Success: ',
        'copy.success': 'Copied',
        'copy.failed': 'Copy failed',
        'copy.hash': 'Copy',
        'copy.code': 'Copy Code',

        // Theme
        'theme.light': 'Switch to Day Mode',
        'theme.dark': 'Switch to Night Mode',
    }
};

// i18n 翻译函数
function T(key) {
    const lang = currentLang;
    if (translationsClips[lang] && translationsClips[lang][key]) {
        return translationsClips[lang][key];
    }
    // 降级到中文 (或任何默认语言)
    return translationsClips['zh'][key] || key;
}


// Tab switching functionality
function switchTab(tabName) {
    // Remove active class from all tabs and content
    document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));

    // Add active class to selected tab and content
    document.querySelector(`[data-tab="${tabName}"]`).classList.add('active');
    document.getElementById(`${tabName}-tab`).classList.add('active');
}

// 过期时间工具：根据数值与单位(d/w/m)转换为秒
function getExpireSeconds(valueInputId, unitSelectId) {
    const v = parseInt((document.getElementById(valueInputId) || {}).value, 10);
    const unit = (document.getElementById(unitSelectId) || {}).value || 'd';
    const n = isNaN(v) || v < 1 ? 1 : v;
    const day = 24 * 60 * 60;
    switch (unit) {
        case 'w': return n * 7 * day;
        case 'm': return n * 30 * day; // 粗略按30天
        case 'd':
        default: return n * day;
    }
}

// Add event listeners for tab buttons
document.querySelectorAll('.tab-button').forEach(button => {
    button.addEventListener('click', () => {
        const tabName = button.getAttribute('data-tab');
        switchTab(tabName);
    });
});

// File size formatter
function formatFileSize(bytes) {
    if (bytes === 0) return `0 ${T('size.bytes')}`;
    const k = 1024;
    // 使用 T() 翻译 sizes 数组中的单位
    const sizes = [T('size.bytes'), T('size.kb'), T('size.mb'), T('size.gb')];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 计算文件SHA-256（分片流式）
async function computeFileSHA256(file, onProgress) {
    // 使用 CryptoJS 增量哈希
    const chunkSize = 4 * 1024 * 1024; // 4MB
    const hasher = CryptoJS.algo.SHA256.create();
    const total = file.size;
    let offset = 0;

    // 兼容的 ArrayBuffer -> WordArray 转换
    function arrayBufferToWordArray(ab) {
        // CryptoJS 4.x 可接受 TypedArray，优先用该路径
        try {
            return CryptoJS.lib.WordArray.create(new Uint8Array(ab));
        } catch (_) {
            // 兜底实现
            const u8 = new Uint8Array(ab);
            const words = [];
            for (let i = 0; i < u8.length; i += 4) {
                words.push(((u8[i] || 0) << 24) | ((u8[i + 1] || 0) << 16) | ((u8[i + 2] || 0) << 8) | (u8[i + 3] || 0));
            }
            return CryptoJS.lib.WordArray.create(words, u8.length);
        }
    }

    while (offset < total) {
        const end = Math.min(offset + chunkSize, total);
        const blob = file.slice(offset, end);
        const arrayBuffer = await new Response(blob).arrayBuffer();

        // 将 ArrayBuffer 转为 WordArray
        const wordArray = arrayBufferToWordArray(arrayBuffer);
        hasher.update(wordArray);

        offset = end;
        if (typeof onProgress === 'function' && total > 0) {
            onProgress((offset / total) * 100);
        }
        // 让出事件循环，避免长时间阻塞UI
        await new Promise(r => setTimeout(r, 0));
    }

    const hash = hasher.finalize().toString(CryptoJS.enc.Hex);
    return hash;
}

// 简单缓存，避免重复计算同一文件的哈希
let lastHashCache = { sig: null, value: null };
function fileSignature(f) {
    // 通过 name/size/lastModified 组合判定是否同一文件
    return [f && f.name, f && f.size, f && f.lastModified].join('::');
}

// File input change handler
document.getElementById('file').addEventListener('change', function (event) {
    const file = event.target.files[0];
    const fileInfo = document.getElementById('fileInfo');
    const formEl = document.getElementById('uploadFileForm');
    const submitButton = formEl ? formEl.querySelector('button[type="submit"]') : null;
    const fileHashSpan = document.getElementById('fileHash');
    const progressText = document.getElementById('progressText');

    if (file) {
        document.getElementById('fileName').textContent = file.name;
        document.getElementById('fileSize').textContent = formatFileSize(file.size);
        if (fileHashSpan) fileHashSpan.textContent = T('info.calculating'); // 翻译
        lastHashCache = { sig: null, value: null };
        fileInfo.style.display = 'block';

        // 禁用上传按钮并开始计算哈希，期间显示进度
        if (submitButton) submitButton.disabled = true;
        updateProgress(0);
        if (progressText) progressText.textContent = `${T('info.hashing')} 0%`; // 翻译

        const sig = fileSignature(file);
        (async () => {
            try {
                const hash = await computeFileSHA256(file, p => {
                    updateProgress(p);
                    if (progressText) progressText.textContent = `${T('info.hashing')} ` + Math.round(p) + '%'; // 翻译
                });
                // 如果用户在计算期间更换了文件，则丢弃结果
                const current = (document.getElementById('file') || {}).files ? document.getElementById('file').files[0] : null;
                if (!current || fileSignature(current) !== sig) return;

                if (fileHashSpan) fileHashSpan.textContent = hash;
                lastHashCache = { sig, value: hash };
            } finally {
                const progressContainer = document.getElementById('progressContainer');
                if (progressContainer) progressContainer.style.display = 'none';
                if (submitButton) submitButton.disabled = false;
            }
        })();
    } else {
        fileInfo.style.display = 'none';
        if (submitButton) submitButton.disabled = false;
    }
});

// 简单的HTML转义，避免文件名中包含HTML字符造成渲染问题
function escapeHtml(str) {
    if (str == null) return '';
    return String(str).replace(/[&<>"']/g, function (c) {
        return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] || c;
    });
}

// 显示历史记录
function loadHistory() {
    const historyList = document.getElementById("historyList");
    historyList.innerHTML = "";
    const items = JSON.parse(localStorage.getItem("clipHistory") || "[]");
    // 更新清空按钮可用状态
    const clearBtn = document.getElementById('clearHistoryBtn');
    if (clearBtn) {
        clearBtn.textContent = T('history.clearBtn'); // 翻译按钮文本
        clearBtn.disabled = items.length === 0;
    }
    items.forEach(item => {
        const li = document.createElement("li");
        const url = `/api/clip/get/${item.code}`;
        let icon, typeText;

        switch (item.type) {
            case 'file':
                icon = '📁';
                typeText = T('history.file'); // 翻译
                break;
            case 'link':
                icon = '🔗';
                typeText = T('history.link'); // 翻译
                break;
            default:
                icon = '📝';
                typeText = T('history.text'); // 翻译
        }

        const filenameText = (item.type === 'file' && item.filename) ? ` - ${escapeHtml(item.filename)}` : '';
        li.innerHTML = `${icon} <a href="${url}" target="_blank">${item.code}</a> ${typeText}${filenameText}`;
        historyList.appendChild(li);
    });
}

// 添加历史记录
function addToHistory(code, type = 'text', meta) {
    const items = JSON.parse(localStorage.getItem("clipHistory") || "[]");
    const entry = { code, type };
    if (meta && typeof meta === 'object') {
        Object.assign(entry, meta);
    }
    items.unshift(entry);
    if (items.length > 10) items.pop(); // 最多保留10条
    localStorage.setItem("clipHistory", JSON.stringify(items));
    loadHistory();
}

// 结果展示与复制提取码
let lastCodeForCopy = null;
function showResult(html, code) {
    const box = document.getElementById('result');
    const body = document.getElementById('resultBody');
    body.innerHTML = html;
    box.style.display = 'block';
    lastCodeForCopy = code || null;
    const btn = document.getElementById('copyCodeBtn');
    if (btn) btn.disabled = !lastCodeForCopy;
}
function showError(text) {
    showResult(`<span style="color:#ffb4b4;">${text}</span>`);
}

async function copyText(text) {
    try {
        await navigator.clipboard.writeText(text);
        return true;
    } catch (_) {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        let ok = false;
        try { ok = document.execCommand('copy'); } catch (e) { ok = false; }
        document.body.removeChild(ta);
        return ok;
    }
}

// 更新进度条
function updateProgress(percent) {
    const progressContainer = document.getElementById('progressContainer');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');

    progressContainer.style.display = 'block';
    progressFill.style.width = percent + '%';
    progressText.textContent = Math.round(percent) + '%';
}

// 文本表单提交逻辑
document.getElementById("createClipForm").addEventListener("submit", function (event) {
    event.preventDefault();

    const fd = new FormData(this);
    fd.set('expire', String(getExpireSeconds('expire', 'expireUnit')));

    fetch(this.action, {
        method: "POST",
        body: fd
    })
        .then(async response => {
            if (response.status === 413) {
                showError(T('error.sizeLimit')); // 翻译
                return;
            }
            let data;
            try {
                data = await response.json();
            } catch (e) {
                showError(T('error.parseResponse')); // 翻译
                return;
            }
            if (response.ok && data.code) {
                const codeUrl = "/api/clip/get/" + data.code;
                showResult(`${T('result.success')}<a href="${codeUrl}" target="_blank">${data.code}</a>`, data.code); // 翻译
                addToHistory(data.code, 'text');
            } else {
                showError(`${T('error.unknown')} ` + (data && data.error ? data.error : T('error.requestFailed'))); // 翻译
            }
        })
        .catch(error => {
            showError(`${T('error.unknown')} ` + error); // 翻译
        });
});

// 短链接表单提交逻辑
document.getElementById("createLinkForm").addEventListener("submit", function (event) {
    event.preventDefault();

    const fd = new FormData(this);
    fd.set('expire', String(getExpireSeconds('linkExpire', 'linkExpireUnit')));

    fetch(this.action, {
        method: "POST",
        body: fd
    })
        .then(async response => {
            if (response.status === 413) {
                showError(T('error.linkSizeLimit')); // 翻译
                return;
            }
            let data;
            try {
                data = await response.json();
            } catch (e) {
                showError(T('error.parseResponse')); // 翻译
                return;
            }
            if (response.ok && data.code) {
                const codeUrl = "/api/clip/get/" + data.code;
                showResult(`${T('result.success')}<a href="${codeUrl}" target="_blank">${data.code}</a>`, data.code); // 翻译
                addToHistory(data.code, 'link');
            } else {
                showError(`${T('error.unknown')} ` + (data && data.error ? data.error : T('error.requestFailed'))); // 翻译
            }
        })
        .catch(error => {
            showError(`${T('error.unknown')} ` + error); // 翻译
        });
});

// 文件上传表单提交逻辑
document.getElementById("uploadFileForm").addEventListener("submit", function (event) {
    event.preventDefault();

    const formEl = this;
    const submitButton = formEl.querySelector('button[type="submit"]');
    const originalText = submitButton.textContent;
    submitButton.textContent = T('info.prepare'); // 翻译
    submitButton.disabled = true;

    const fileInput = document.getElementById('file');
    const file = fileInput.files[0];
    if (!file) {
        showError(T('info.selectFile')); // 翻译
        submitButton.textContent = originalText;
        submitButton.disabled = false;
        return;
    }

    // 先计算哈希
    updateProgress(0);
    const progressText = document.getElementById('progressText');
    const fileHashSpan = document.getElementById('fileHash');
    progressText.textContent = `${T('info.hashing')} 0%`; // 翻译

    const sig = fileSignature(file);
    const doHash = async () => {
        if (lastHashCache.sig === sig && lastHashCache.value) {
            return lastHashCache.value;
        }
        const hash = await computeFileSHA256(file, p => {
            updateProgress(p);
            progressText.textContent = `${T('info.hashing')} ` + Math.round(p) + '%'; // 翻译
        });
        lastHashCache = { sig, value: hash };
        return hash;
    };

    (async () => {
        try {
            const hash = await doHash();
            fileHashSpan.textContent = hash;

            // 开始上传
            submitButton.textContent = T('info.uploading'); // 翻译
            updateProgress(0);

            const xhr = new XMLHttpRequest();
            const formData = new FormData(formEl);
            formData.set('expire', String(getExpireSeconds('fileExpire', 'fileExpireUnit')));
            formData.append('client_sha256', hash);

            xhr.upload.addEventListener('progress', function (e) {
                if (e.lengthComputable) {
                    const percentComplete = (e.loaded / e.total) * 100;
                    updateProgress(percentComplete);
                    progressText.textContent = Math.round(percentComplete) + '%';
                }
            });

            xhr.addEventListener('load', function () {
                if (xhr.status === 413) {
                    showError(T('error.uploadSizeLimit')); // 翻译
                } else if (xhr.status === 200) {
                    try {
                        const data = JSON.parse(xhr.responseText);
                        if (data.code) {
                            const downloadUrl = "/api/clip/get/" + data.code;
                            const instantUploadText = data.instant_upload ? T('info.instantUpload') : ''; // 翻译
                            showResult(`${T('info.uploadSuccess')}${instantUploadText}：<a href="${downloadUrl}" target="_blank">${data.code}</a>`, data.code); // 翻译
                            addToHistory(data.code, 'file', { filename: file.name });

                            // Reset form
                            formEl.reset();
                            document.getElementById('fileInfo').style.display = 'none';
                            fileHashSpan.textContent = '';
                            lastHashCache = { sig: null, value: null };
                        } else {
                            showError(`${T('error.unknown')} ` + data.error); // 翻译
                        }
                    } catch (e) {
                        showError(T('error.parseUploadResponse')); // 翻译
                    }
                } else {
                    showError(`${T('error.uploadFailed')} ` + xhr.status); // 翻译
                }

                // 隐藏进度条并重置按钮
                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.addEventListener('error', function () {
                showError(T('error.network')); // 翻译
                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.open('POST', formEl.action);
            xhr.send(formData);
        } catch (err) {
            // 哈希失败，降级为直接上传（不带 client_sha256）
            console.warn('Hashing failed, fallback to upload without client hash:', err); // 保持英文，这是开发者警告
            fileHashSpan.textContent = '';

            submitButton.textContent = T('info.uploading'); // 翻译
            updateProgress(0);

            const xhr = new XMLHttpRequest();
            const formData = new FormData(formEl);
            formData.set('expire', String(getExpireSeconds('fileExpire', 'fileExpireUnit')));

            xhr.upload.addEventListener('progress', function (e) {
                if (e.lengthComputable) {
                    const percentComplete = (e.loaded / e.total) * 100;
                    updateProgress(percentComplete);
                    progressText.textContent = Math.round(percentComplete) + '%';
                }
            });

            xhr.addEventListener('load', function () {
                if (xhr.status === 413) {
                    showError(T('error.uploadSizeLimit')); // 翻译
                } else if (xhr.status === 200) {
                    try {
                        const data = JSON.parse(xhr.responseText);
                        if (data.code) {
                            const downloadUrl = "/api/clip/get/" + data.code;
                            const instantUploadText = data.instant_upload ? T('info.instantUpload') : ''; // 翻译
                            showResult(`${T('info.uploadSuccess')}${instantUploadText}：<a href="${downloadUrl}" target="_blank">${data.code}</a>`, data.code); // 翻译
                            addToHistory(data.code, 'file', { filename: file.name });

                            // Reset form
                            formEl.reset();
                            document.getElementById('fileInfo').style.display = 'none';
                            fileHashSpan.textContent = '';
                            lastHashCache = { sig: null, value: null };
                        } else {
                            showError(`${T('error.unknown')} ` + data.error); // 翻译
                        }
                    } catch (e) {
                        showError(T('error.parseUploadResponse')); // 翻译
                    }
                } else {
                    showError(`${T('error.uploadFailed')} ` + xhr.status); // 翻译
                }

                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.addEventListener('error', function () {
                showError(T('error.network')); // 翻译
                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.open('POST', formEl.action);
            xhr.send(formData);
        }
    })();
});

// 应用 i18n 到 HTML 元素
function applyI18nToHtml() {
    // 翻译 Tab 按钮
    document.querySelectorAll('[data-tab-i18n]').forEach(el => {
        const key = el.getAttribute('data-tab-i18n');
        el.textContent = T(key);
    });

    // 翻译普通文本/按钮
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        el.textContent = T(key);
    });

    // 翻译 Placeholder
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        el.placeholder = T(key);
    });

    // 重新加载历史记录，以确保历史记录项的文本被翻译
    loadHistory();
}

// 初始加载历史记录和应用 i18n
loadHistory();
applyI18nToHtml();


// 绑定清空历史按钮
(function bindClearHistory() {
    const btn = document.getElementById('clearHistoryBtn');
    if (!btn) return;
    btn.textContent = T('history.clearBtn'); // 初始设置/翻译
    btn.addEventListener('click', function () {
        if (confirm(T('history.clearConfirm'))) { // 翻译确认文本
            localStorage.removeItem('clipHistory');
            loadHistory();
            const result = document.getElementById('result');
            const body = document.getElementById('resultBody');
            if (result && body) {
                result.style.display = 'block';
                body.textContent = T('history.cleared'); // 翻译清空成功文本
                lastCodeForCopy = null;
                const cc = document.getElementById('copyCodeBtn');
                if (cc) cc.disabled = true;
            }
        }
    });
})();

// 主题切换与持久化
(function setupThemeToggle() {
    const btn = document.getElementById('themeToggle');
    function applyTheme(mode) {
        const body = document.body;
        if (mode === 'light') {
            body.classList.add('light-mode');
            if (btn) {
                btn.textContent = '🌙';
                btn.setAttribute('aria-label', T('theme.dark')); // 翻译
            }
        } else {
            body.classList.remove('light-mode');
            if (btn) {
                btn.textContent = '🌞';
                btn.setAttribute('aria-label', T('theme.light')); // 翻译
            }
        }
    }
    const saved = localStorage.getItem('theme') || 'dark';
    applyTheme(saved);
    if (btn) {
        btn.addEventListener('click', () => {
            const next = document.body.classList.contains('light-mode') ? 'dark' : 'light';
            localStorage.setItem('theme', next);
            applyTheme(next);
        });
    }
})();

// 绑定复制与拖拽
(function bindExtras() {
    const copyHashBtn = document.getElementById('copyHashBtn');
    if (copyHashBtn) {
        copyHashBtn.textContent = T('copy.hash'); // 初始文本翻译
        copyHashBtn.addEventListener('click', async function () {
            const hash = (document.getElementById('fileHash') || {}).textContent || '';
            if (!hash) return;
            const ok = await copyText(hash);
            copyHashBtn.textContent = ok ? T('copy.success') : T('copy.failed'); // 翻译
            setTimeout(() => copyHashBtn.textContent = T('copy.hash'), 1200); // 翻译
        });
    }
    const copyCodeBtn = document.getElementById('copyCodeBtn');
    if (copyCodeBtn) {
        copyCodeBtn.textContent = T('copy.code'); // 初始文本翻译
        copyCodeBtn.disabled = !lastCodeForCopy;
        copyCodeBtn.addEventListener('click', async function () {
            if (!lastCodeForCopy) return;
            const ok = await copyText(String(lastCodeForCopy));
            copyCodeBtn.textContent = ok ? T('copy.success') : T('copy.failed'); // 翻译
            setTimeout(() => copyCodeBtn.textContent = T('copy.code'), 1200); // 翻译
        });
    }

    const dz = document.getElementById('dropzone');
    const input = document.getElementById('file');
    if (dz && input) {
        const onEnter = (e) => { e.preventDefault(); dz.classList.add('dragover'); };
        const onLeave = (e) => { e.preventDefault(); dz.classList.remove('dragover'); };
        dz.addEventListener('dragenter', onEnter);
        dz.addEventListener('dragover', onEnter);
        dz.addEventListener('dragleave', onLeave);
        dz.addEventListener('drop', function (e) {
            e.preventDefault();
            dz.classList.remove('dragover');
            if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
                try { input.files = e.dataTransfer.files; } catch (_) { }
                // 触发变更，刷新文件信息
                const evt = new Event('change');
                input.dispatchEvent(evt);
            }
        });
    }
})();