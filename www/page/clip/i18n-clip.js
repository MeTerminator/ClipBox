// --- i18n ---

const translations = {
    'en': {
        // HTML Title
        'documentTitle': 'Clip Clipboard | ClipBox',
        // Header
        'themeToggleLabel': 'Toggle Theme',
        'headerTitle': '📦 Clip Clipboard',
        'headerSubtitle': 'Easily transfer text, links, and files across multiple devices',
        'backHome': 'Back to Homepage',
        // Tabs
        'fileUploadTab': 'File Upload',
        'textClipboardTab': 'Text Clipboard',
        'shortLinkTab': 'Short Link',
        // File Tab
        'selectFileLabel': 'Select File',
        'dropzoneInstruction': 'Drag and drop file here, or click to select',
        'fileName': 'File Name:',
        'fileSize': 'File Size:',
        'fileHash': 'SHA-256:',
        'copy': 'Copy',
        'maxDownloadsLabel': 'Max Downloads',
        'expireTimeLabel': 'Expiration Time',
        'days': 'Days',
        'weeks': 'Weeks',
        'months': 'Months',
        'uploadFileButton': 'Upload File',
        // Text Tab
        'textContentLabel': 'Text Content',
        'maxAccessLabel': 'Max Accesses',
        'createButton': 'Create',
        // Link Tab
        'linkAddressLabel': 'Link Address',
        'createShortLinkButton': 'Create Short Link',
        // Result Box
        'resultHeader': 'Notice',
        'copyCodeButton': 'Copy Extraction Code',
        // History
        'historyHeader': 'History',
        'clearHistoryButton': 'Clear History',
    },
    // 中文翻译
    'zh': {
        // HTML Title
        'documentTitle': 'Clip 剪贴板 | ClipBox',
        // Header
        'themeToggleLabel': '切换主题',
        'headerTitle': '📦 Clip 剪贴板',
        'headerSubtitle': '轻松在多端间传文本、链接和文件',
        'backHome': '返回主页',
        // Tabs
        'fileUploadTab': '文件上传',
        'textClipboardTab': '文本剪贴板',
        'shortLinkTab': '短链接',
        // File Tab
        'selectFileLabel': '选择文件',
        'dropzoneInstruction': '拖拽文件到此处，或点击选择',
        'fileName': '文件名:',
        'fileSize': '文件大小:',
        'fileHash': 'SHA-256:',
        'copy': '复制',
        'maxDownloadsLabel': '最大下载次数',
        'expireTimeLabel': '过期时间',
        'days': '天',
        'weeks': '周',
        'months': '月',
        'uploadFileButton': '上传文件',
        // Text Tab
        'textContentLabel': '文本内容',
        'maxAccessLabel': '最大访问次数',
        'createButton': '创建',
        // Link Tab
        'linkAddressLabel': '链接地址',
        'createShortLinkButton': '创建短链接',
        // Result Box
        'resultHeader': '提示',
        'copyCodeButton': '复制提取码',
        // History
        'historyHeader': '历史记录',
        'clearHistoryButton': '清空历史',
    }
};

// 1. 获取浏览器语言
const browserLang = navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';

// 2. 尝试从 localStorage 获取用户首选语言，如果没有，使用浏览器语言
let currentLang = localStorage.getItem('userLang') || browserLang;

const langToggle = document.getElementById('langToggle');

/**
 * 根据当前语言设置更新所有标记的元素
 * @param {string} lang - 要应用的语言代码 ('zh' 或 'en')
 */
function applyTranslations(lang) {
    const translationSet = translations[lang];
    if (!translationSet) return;

    // 1. 更新所有 data-i18n 属性的 innerText
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (translationSet[key]) {
            el.innerText = translationSet[key];
        }
    });

    // 2. 更新所有 data-i18n-prefix 的文本
    document.querySelectorAll('[data-i18n-prefix]').forEach(el => {
        const key = el.getAttribute('data-i18n-prefix');
        if (translationSet[key]) {
            el.firstChild.textContent = translationSet[key] + ' ';
        }
    });

    // 3. 更新所有 data-i18n-aria-label 属性
    document.querySelectorAll('[data-i18n-aria-label]').forEach(el => {
        const key = el.getAttribute('data-i18n-aria-label');
        if (translationSet[key]) {
            el.setAttribute('aria-label', translationSet[key]);
        }
    });

    // 4. 更新文档标题
    const titleEl = document.querySelector('title');
    const titleKey = titleEl.getAttribute('data-i18n');
    if (titleKey && translationSet[titleKey]) {
        titleEl.innerText = translationSet[titleKey];
    }

    // 5. 更新语言切换按钮的文本和当前文档的 lang 属性
    if (lang === 'zh') {
        langToggle.innerText = 'Eng';
        document.documentElement.lang = 'zh-cn';
    } else {
        langToggle.innerText = '中';
        document.documentElement.lang = 'en';
    }

    // 6. 更新 localStorage
    localStorage.setItem('userLang', lang);
}

// 语言切换事件监听
if (langToggle) {
    langToggle.addEventListener('click', () => {
        // 切换语言
        currentLang = currentLang === 'zh' ? 'en' : 'zh';
        applyTranslations(currentLang);
    });
}


// 页面加载时立即应用一次翻译
document.addEventListener('DOMContentLoaded', () => {
    applyTranslations(currentLang);
});
