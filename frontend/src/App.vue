<template>
  <header class="app-header">
    <router-link to="/" class="brand">
      <div class="logo"></div>
      <span class="title">ClipBox</span>
    </router-link>
    <div class="actions">
      <select v-model="$i18n.locale" @change="saveLang" class="lang-select">
        <option value="en">English</option>
        <option value="zh">简体中文</option>
      </select>
      <button class="theme-toggle" @click="toggleTheme">
        {{ isLight ? '🌙' : '☀️' }}
      </button>
    </div>
  </header>

  <main class="container">
    <router-view v-slot="{ Component }">
      <transition name="fade" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
  </main>

  <footer class="app-footer">
    <p>&copy; {{ new Date().getFullYear() }} <a href="https://github.com/MeTerminator/ClipBox"
        target="_blank">ClipBox</a></p>
  </footer>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const isLight = ref(false)

onMounted(() => {
  const theme = localStorage.getItem('theme') || 'dark'
  isLight.value = theme === 'light'
  if (isLight.value) document.body.classList.add('light-mode')
})

const toggleTheme = () => {
  isLight.value = !isLight.value
  if (isLight.value) {
    document.body.classList.add('light-mode')
    localStorage.setItem('theme', 'light')
  } else {
    document.body.classList.remove('light-mode')
    localStorage.setItem('theme', 'dark')
  }
}

const saveLang = (e) => {
  localStorage.setItem('lang', e.target.value)
}
</script>

<style scoped>
.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
  background: rgba(17, 24, 39, 0.5);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border);
}

.light-mode .app-header {
  background: rgba(255, 255, 255, 0.5);
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-decoration: none;
  color: var(--fg);
  font-weight: bold;
  font-size: 1.25rem;
}

.logo {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(135deg, var(--primary), var(--accent));
}

.actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.lang-select {
  background: transparent;
  color: var(--fg);
  border: 1px solid var(--border);
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  outline: none;
}

.lang-select option {
  background: var(--bg);
}

.theme-toggle {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.25rem;
  color: var(--fg);
}

.app-footer {
  text-align: center;
  padding: 1.5rem;
  color: var(--muted);
  font-size: 0.875rem;
}

.app-footer a {
  color: inherit;
  text-decoration: none;
}

.app-footer a:hover {
  text-decoration: underline;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
