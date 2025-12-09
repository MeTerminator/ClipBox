// 默认语言
const defaultLang = 'en';

// 存储所有语言的翻译文本
let translations = {};

/**
 * 加载指定的语言文件
 * @param {string} lang - 语言代码 (e.g., 'en', 'zh')
 * @returns {Promise<Object>} 包含翻译文本的 Promise
 */
async function loadTranslations(lang) {
    if (translations[lang]) {
        return translations[lang];
    }
    try {
        const response = await fetch(`./locales/${lang}.json`);
        if (!response.ok) {
            throw new Error(`Failed to load ${lang} locale`);
        }
        const data = await response.json();
        translations[lang] = data;
        return data;
    } catch (error) {
        console.error("Error loading translations:", error);
        // 如果加载失败，尝试加载默认语言
        if (lang !== defaultLang) {
            return loadTranslations(defaultLang);
        }
        return {};
    }
}

/**
 * 应用翻译到页面上的所有元素
 * @param {string} lang - 目标语言代码
 * @param {Object} dictionary - 翻译字典
 */
function applyTranslations(lang, dictionary) {
    // 1. 设置 HTML 的 lang 和 data-lang 属性
    document.documentElement.lang = lang;
    document.documentElement.dataset.lang = lang;

    // 2. 翻译普通文本内容
    document.querySelectorAll('[data-i18n]').forEach(element => {
        const key = element.dataset.i18n;
        if (dictionary[key]) {
            element.textContent = dictionary[key];
        }
    });

    // 3. 翻译 placeholder 属性
    document.querySelectorAll('[data-i18n-placeholder]').forEach(element => {
        const key = element.dataset.i18nPlaceholder;
        if (dictionary[key]) {
            element.placeholder = dictionary[key];
        }
    });

    // 4. 翻译 aria-label 属性
    document.querySelectorAll('[data-i18n-aria-label]').forEach(element => {
        const key = element.dataset.i18nAriaLabel;
        if (dictionary[key]) {
            element.setAttribute('aria-label', dictionary[key]);
        }
    });

    // 5. 更新语言切换器
    const switcher = document.getElementById('language-switcher');
    if (switcher) {
        switcher.value = lang;
    }
}

/**
 * 语言切换的主函数，可以由 HTML 中的 onchange 事件调用
 * @param {string} lang - 目标语言代码
 */
async function changeLanguage(lang) {
    const dictionary = await loadTranslations(lang);
    applyTranslations(lang, dictionary);
    // 可选：将选择的语言存储到 localStorage 中
    localStorage.setItem('clipboxLang', lang);
}

/**
 * 初始化：根据浏览器或存储的偏好设置语言
 */
async function initializeLanguage() {
    // 1. 尝试从 localStorage 读取用户上次选择的语言
    let userLang = localStorage.getItem('clipboxLang');

    // 2. 如果没有，尝试获取浏览器偏好的语言
    if (!userLang) {
        const browserLang = navigator.language.toLowerCase();
        // 简单匹配，例如 'zh-cn' 或 'en-us'
        if (browserLang.startsWith('zh')) {
            userLang = 'zh';
        } else {
            userLang = defaultLang;
        }
    }

    // 3. 切换到确定的语言
    await changeLanguage(userLang);
}

// 页面加载完成后调用初始化函数
document.addEventListener('DOMContentLoaded', initializeLanguage);