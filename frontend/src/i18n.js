import { createI18n } from 'vue-i18n'

const messages = {
  en: {
    home: {
      title: 'Welcome to ClipBox',
      desc: 'Enter 5-digit pickup code to quickly get text, links, or files',
      pickupCode: 'Enter the 5-digit pickup code, e.g., 12345',
      pickupBtn: 'Pick Up Now',
      tips: 'Tip: Press Enter to pick up directly. Short links will be redirected automatically, text will be displayed directly, and files will start downloading.',
      clipTitle: 'Clipboard',
      clipDesc: 'Create pickup codes, share text, links, and files across devices'
    },
    clip: {
      text: 'Text',
      link: 'Short Link',
      file: 'File',
      history: 'History',
      create: 'Create',
      upload: 'Upload',
      selectFile: 'Select File',
      contentLabel: 'Content',
      linkUrl: 'URL',
      countLabel: 'Max access count',
      expireLabel: 'Expires in',
      success: 'Success: ',
      error: 'Error: ',
      clearHistory: 'Clear History',
      backToHome: 'Back to Home',
      copy: 'Copy',
      copied: 'Copied!',
      redirectNotice: 'File upload hosting, will authorize to: {domain}',
      redirectMsg: 'Upload successful. Redirect to {domain}?',
      redirectBtn: 'Redirect Now',
      stayBtn: 'Stay Here',
      units: {
        seconds: 'Seconds',
        minutes: 'Minutes',
        hours: 'Hours',
        days: 'Days',
        weeks: 'Weeks',
        years: 'Years'
      }
    },
    nav: {
      themeLight: 'Light Mode',
      themeDark: 'Dark Mode'
    }
  },
  zh: {
    home: {
      title: '欢迎来到 ClipBox',
      desc: '输入 5 位取件码，快速获取文本、短链接或文件',
      pickupCode: '请输入 5 位取件码，例如 12345',
      pickupBtn: '立即取件',
      tips: '提示：按回车键直接取件。短链接将自动跳转，文本直接显示，文件将开始下载。',
      clipTitle: '剪贴板',
      clipDesc: '创建取件码，跨设备分享文本、短链接与文件'
    },
    clip: {
      text: '文本',
      link: '短链接',
      file: '文件',
      history: '历史',
      create: '创建',
      upload: '上传',
      selectFile: '选择文件',
      contentLabel: '内容',
      linkUrl: '链接地址',
      countLabel: '最大访问次数',
      expireLabel: '过期时间',
      success: '创建成功：',
      error: '错误：',
      clearHistory: '清空历史',
      backToHome: '返回主页',
      copy: '复制',
      copied: '已复制!',
      redirectNotice: '文件上传托管，将授权至：{domain}',
      redirectMsg: '上传成功。是否跳转至 {domain}？',
      redirectBtn: '立即跳转',
      stayBtn: '留在当前页',
      units: {
        seconds: '秒',
        minutes: '分',
        hours: '时',
        days: '天',
        weeks: '周',
        years: '年'
      }
    },
    nav: {
      themeLight: '日间模式',
      themeDark: '夜间模式'
    }
  }
}

export default createI18n({
  locale: localStorage.getItem('lang') || 'zh',
  fallbackLocale: 'en',
  messages
})
