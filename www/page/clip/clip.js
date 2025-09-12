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
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
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
document.getElementById('file').addEventListener('change', function(event) {
    const file = event.target.files[0];
    const fileInfo = document.getElementById('fileInfo');
    const formEl = document.getElementById('uploadFileForm');
    const submitButton = formEl ? formEl.querySelector('button[type="submit"]') : null;
    const fileHashSpan = document.getElementById('fileHash');
    const progressText = document.getElementById('progressText');
    
    if (file) {
        document.getElementById('fileName').textContent = file.name;
        document.getElementById('fileSize').textContent = formatFileSize(file.size);
        if (fileHashSpan) fileHashSpan.textContent = '计算中';
        lastHashCache = { sig: null, value: null };
        fileInfo.style.display = 'block';

        // 禁用上传按钮并开始计算哈希，期间显示进度
        if (submitButton) submitButton.disabled = true;
        updateProgress(0);
        if (progressText) progressText.textContent = '计算哈希 0%';

        const sig = fileSignature(file);
        (async () => {
            try {
                const hash = await computeFileSHA256(file, p => {
                    updateProgress(p);
                    if (progressText) progressText.textContent = '计算哈希 ' + Math.round(p) + '%';
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
        clearBtn.disabled = items.length === 0;
    }
    items.forEach(item => {
        const li = document.createElement("li");
        const url = `/api/clip/get/${item.code}`;
        let icon, typeText;
        
        switch(item.type) {
            case 'file':
                icon = '📁';
                typeText = '(文件)';
                break;
            case 'link':
                icon = '🔗';
                typeText = '(短链接)';
                break;
            default:
                icon = '📝';
                typeText = '(文本)';
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
                showError("❌ 错误: 文本内容过大，已超过服务器限制");
                return;
            }
            let data;
            try {
                data = await response.json();
            } catch (e) {
                showError("❌ 错误: 无法解析服务器响应");
                return;
            }
            if (response.ok && data.code) {
                const codeUrl = "/api/clip/get/" + data.code;
                showResult(`✅ 创建成功：<a href="${codeUrl}" target="_blank">${data.code}</a>`, data.code);
                addToHistory(data.code, 'text');
            } else {
                showError("❌ 错误: " + (data && data.error ? data.error : "请求失败"));
            }
        })
        .catch(error => {
            showError("❌ 错误: " + error);
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
                showError("❌ 错误: 链接内容过大，已超过服务器限制");
                return;
            }
            let data;
            try {
                data = await response.json();
            } catch (e) {
                showError("❌ 错误: 无法解析服务器响应");
                return;
            }
            if (response.ok && data.code) {
                const codeUrl = "/api/clip/get/" + data.code;
                showResult(`✅ 短链接创建成功：<a href="${codeUrl}" target="_blank">${data.code}</a>`, data.code);
                addToHistory(data.code, 'link');
            } else {
                showError("❌ 错误: " + (data && data.error ? data.error : "请求失败"));
            }
        })
        .catch(error => {
            showError("❌ 错误: " + error);
        });
});

// 文件上传表单提交逻辑
document.getElementById("uploadFileForm").addEventListener("submit", function (event) {
    event.preventDefault();

    const formEl = this;
    const submitButton = formEl.querySelector('button[type="submit"]');
    const originalText = submitButton.textContent;
    submitButton.textContent = '准备中...';
    submitButton.disabled = true;

    const fileInput = document.getElementById('file');
    const file = fileInput.files[0];
    if (!file) {
        showError("❌ 请选择文件");
        submitButton.textContent = originalText;
        submitButton.disabled = false;
        return;
    }

    // 先计算哈希
    updateProgress(0);
    const progressText = document.getElementById('progressText');
    const fileHashSpan = document.getElementById('fileHash');
    progressText.textContent = '哈希 0%';

    const sig = fileSignature(file);
    const doHash = async () => {
        if (lastHashCache.sig === sig && lastHashCache.value) {
            return lastHashCache.value;
        }
        const hash = await computeFileSHA256(file, p => {
            updateProgress(p);
            progressText.textContent = '哈希 ' + Math.round(p) + '%';
        });
        lastHashCache = { sig, value: hash };
        return hash;
    };

    (async () => {
        try {
            const hash = await doHash();
            fileHashSpan.textContent = hash;

            // 开始上传
            submitButton.textContent = '上传中...';
            updateProgress(0);

            const xhr = new XMLHttpRequest();
            const formData = new FormData(formEl);
            formData.set('expire', String(getExpireSeconds('fileExpire', 'fileExpireUnit')));
            formData.append('client_sha256', hash);

            xhr.upload.addEventListener('progress', function(e) {
                if (e.lengthComputable) {
                    const percentComplete = (e.loaded / e.total) * 100;
                    updateProgress(percentComplete);
                    progressText.textContent = Math.round(percentComplete) + '%';
                }
            });

            xhr.addEventListener('load', function() {
                if (xhr.status === 413) {
                    showError("❌ 上传失败: 文件过大，已超过服务器限制");
                } else if (xhr.status === 200) {
                    try {
                        const data = JSON.parse(xhr.responseText);
                        if (data.code) {
                            const downloadUrl = "/api/clip/get/" + data.code;
                            const instantUploadText = data.instant_upload ? ' (秒传)' : '';
                            showResult(`✅ 上传成功${instantUploadText}：<a href="${downloadUrl}" target="_blank">${data.code}</a>`, data.code);
                            addToHistory(data.code, 'file', { filename: file.name });

                            // Reset form
                            formEl.reset();
                            document.getElementById('fileInfo').style.display = 'none';
                            fileHashSpan.textContent = '';
                            lastHashCache = { sig: null, value: null };
                        } else {
                            showError("❌ 错误: " + data.error);
                        }
                    } catch (e) {
                        showError("❌ 解析响应失败");
                    }
                } else {
                    showError("❌ 上传失败: " + xhr.status);
                }

                // 隐藏进度条并重置按钮
                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.addEventListener('error', function() {
                showError("❌ 网络错误");
                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.open('POST', formEl.action);
            xhr.send(formData);
        } catch (err) {
            // 哈希失败，降级为直接上传（不带 client_sha256）
            console.warn('Hashing failed, fallback to upload without client hash:', err);
            fileHashSpan.textContent = '';

            submitButton.textContent = '上传中...';
            updateProgress(0);

            const xhr = new XMLHttpRequest();
            const formData = new FormData(formEl);
            formData.set('expire', String(getExpireSeconds('fileExpire', 'fileExpireUnit')));

            xhr.upload.addEventListener('progress', function(e) {
                if (e.lengthComputable) {
                    const percentComplete = (e.loaded / e.total) * 100;
                    updateProgress(percentComplete);
                    progressText.textContent = Math.round(percentComplete) + '%';
                }
            });

            xhr.addEventListener('load', function() {
                if (xhr.status === 413) {
                    showError("❌ 上传失败: 文件过大，已超过服务器限制");
                } else if (xhr.status === 200) {
                    try {
                        const data = JSON.parse(xhr.responseText);
                        if (data.code) {
                            const downloadUrl = "/api/clip/get/" + data.code;
                            const instantUploadText = data.instant_upload ? ' (秒传)' : '';
                            showResult(`✅ 上传成功${instantUploadText}：<a href="${downloadUrl}" target="_blank">${data.code}</a>`, data.code);
                            addToHistory(data.code, 'file', { filename: file.name });

                            // Reset form
                            formEl.reset();
                            document.getElementById('fileInfo').style.display = 'none';
                            fileHashSpan.textContent = '';
                            lastHashCache = { sig: null, value: null };
                        } else {
                            showError("❌ 错误: " + data.error);
                        }
                    } catch (e) {
                        showError("❌ 解析响应失败");
                    }
                } else {
                    showError("❌ 上传失败: " + xhr.status);
                }

                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.addEventListener('error', function() {
                showError("❌ 网络错误");
                document.getElementById('progressContainer').style.display = 'none';
                submitButton.textContent = originalText;
                submitButton.disabled = false;
            });

            xhr.open('POST', formEl.action);
            xhr.send(formData);
        }
    })();
});

// 初始加载历史记录
loadHistory();

// 绑定清空历史按钮
(function bindClearHistory() {
    const btn = document.getElementById('clearHistoryBtn');
    if (!btn) return;
    btn.addEventListener('click', function() {
        if (confirm('确定要清空历史吗？')) {
            localStorage.removeItem('clipHistory');
            loadHistory();
            const result = document.getElementById('result');
            const body = document.getElementById('resultBody');
            if (result && body) {
                result.style.display = 'block';
                body.textContent = '🧹 已清空历史记录';
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
                btn.setAttribute('aria-label', '切换到夜间模式');
            }
        } else {
            body.classList.remove('light-mode');
            if (btn) {
                btn.textContent = '🌞';
                btn.setAttribute('aria-label', '切换到日间模式');
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
        copyHashBtn.addEventListener('click', async function() {
            const hash = (document.getElementById('fileHash') || {}).textContent || '';
            if (!hash) return;
            const ok = await copyText(hash);
            copyHashBtn.textContent = ok ? '已复制' : '复制失败';
            setTimeout(() => copyHashBtn.textContent = '复制', 1200);
        });
    }
    const copyCodeBtn = document.getElementById('copyCodeBtn');
    if (copyCodeBtn) {
        copyCodeBtn.disabled = !lastCodeForCopy;
        copyCodeBtn.addEventListener('click', async function() {
            if (!lastCodeForCopy) return;
            const ok = await copyText(String(lastCodeForCopy));
            copyCodeBtn.textContent = ok ? '已复制' : '复制失败';
            setTimeout(() => copyCodeBtn.textContent = '复制提取码', 1200);
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
        dz.addEventListener('drop', function(e) {
            e.preventDefault();
            dz.classList.remove('dragover');
            if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
                try { input.files = e.dataTransfer.files; } catch (_) {}
                // 触发变更，刷新文件信息
                const evt = new Event('change');
                input.dispatchEvent(evt);
            }
        });
    }
})();