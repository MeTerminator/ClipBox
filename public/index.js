
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

    // 自动根据浏览器设置应用主题
    const userPref = localStorage.getItem("ui.darkmode");
    let darkmode;

    if (userPref === null) {
        // 用户没有手动设置过，跟随系统
        darkmode = window.matchMedia('(prefers-color-scheme: dark)').matches;
    } else {
        darkmode = userPref === "true";
    }

    toggleTheme(darkmode);

    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
        const userPref = localStorage.getItem("ui.darkmode");
        if (userPref === null) {
            toggleTheme(e.matches);
        }
    });

})();