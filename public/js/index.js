
class PersonCard extends HTMLElement {
    static get observedAttributes() {
        return ['avatar', 'name', 'description1', 'description2', 'link'];
    }

    constructor() {
        super();
        this._attributes = {
            avatar: '',
            name: '',
            description1: '',
            description2: '',
            link: ''
        };
        this.render();
    }

    attributeChangedCallback(name, oldValue, newValue) {
        if (oldValue !== newValue) {
            this._attributes[name] = newValue || '';
            this.render();
        }
    }

    _escapeHTML(str) {
        return str.replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    render() {
        const attrs = Object.fromEntries(
            Object.entries(this._attributes).map(([k, v]) => [k, this._escapeHTML(v)])
        );

        const descriptions = [];
        if (attrs.description1) descriptions.push(
            `<span style="font-size: 14px; color: #666;">${attrs.description1}</span><br>`
        );
        if (attrs.description2) descriptions.push(
            `<span style="font-size: 14px; color: #666;">${attrs.description2}</span><br>`
        );

        const link = attrs.link ? `
        <a href="${attrs.link}" 
           target="_blank" 
           style="float:right; margin-right: 10px; margin-left: auto;" 
           rel="nofollow noopener" aria-label="点击访问该站点">
          <i class="fa fa-angle-right" style="font-weight: bold; font-size: 34px"></i>
        </a>
      ` : '';

        this.innerHTML = `
  <div class="person-card">
    <div>
      <img src="${attrs.avatar}" alt="用户头像">
      <p>
        <span style="font-size: 20px;">${attrs.name}</span><br>
        ${descriptions.join('\n')}
      </p>
      ${link}
    </div>
  </div>`;
    }
}

customElements.define('person-card', PersonCard);

// 功能性方法
function openInNewTab(url) {
    window.open(url, '_blank').focus();
}

function toggleTheme(mode = null) {
    const isDarkMode = mode === null ? document.documentElement.classList.contains("darkmode") : mode;
    if (isDarkMode) {
        document.documentElement.classList.remove("darkmode");
        localStorage.setItem("ui.darkmode", "false");
    }
    else {
        document.documentElement.classList.add("darkmode");
        localStorage.setItem("ui.darkmode", "true");
    }
}

(() => {
    // 在菜单激活时运行
    function setMenuAnimationDelay() {
        document.querySelectorAll('.nav-main.active .nav-links li:not(:first-child)').forEach((li, index) => {
            li.style.animationDelay = `${0.1 * (index + 1)}s`;
        });
    }

    // 导航栏在窄视口设备下的自动折叠
    const menuBtn = document.querySelector('.nav-menu-btn');
    const navMain = document.querySelector('.nav-main');

    if (!menuBtn || !navMain) {
        // 没有导航栏，跑路
        return;
    }

    menuBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        setMenuAnimationDelay();
        navMain.classList.toggle('active');
    });

    document.addEventListener('click', (e) => {
        if (navMain.classList.contains('active')) {
            navMain.classList.remove('active');
        }
    });

    navMain.addEventListener('click', (e) => {
        e.stopPropagation();
    });
})();

/* ====== 当前时间卡片的逻辑 ====== */
function updateTime() {
    const now = new Date();
    const str = now.toLocaleTimeString('zh-CN', { hour12: false });
    const el = document.getElementById('currentTime');
    if (el) el.textContent = str;
}
/* 等页面加载完再启动计时器，避免找不到元素 */
window.addEventListener('DOMContentLoaded', () => {
    updateTime();
    setInterval(updateTime, 1000); // 每秒刷新一次
});
function handleClipCodeInput(value) {
    const cleaned = value.replace(/\D/g, ''); // 只保留数字
    const input = document.getElementById('clipCodeInput');
    input.value = cleaned.slice(0, 4); // 限制为最多4位

    const extractBtnWrapper = document.getElementById('extractButtonWrapper');
    if (cleaned.length === 4) {
        extractBtnWrapper.style.display = 'block';
    } else {
        extractBtnWrapper.style.display = 'none';
    }
}

function extractClipCode() {
    const code = document.getElementById('clipCodeInput').value;
    if (/^\d{4}$/.test(code)) {
        // window.location.href = `/api/clip/get/${code}`;
        openInNewTab(`/api/clip/get/${code}`);
    }
}