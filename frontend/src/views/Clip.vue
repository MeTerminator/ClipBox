<template>
  <div class="glass-card clip-view">
    <div class="header-row">
      <router-link to="/" class="back-link">← {{ $t('clip.backToHome') || 'Back' }}</router-link>
    </div>
    <div v-if="route.query.redirect" class="info-banner">
      {{ $t('clip.redirectNotice', { domain: getDomain(route.query.redirect) }) }}
    </div>
    <div class="tabs">
      <button v-for="tab in ['text', 'link', 'file', 'history']" :key="tab"
        :class="['tab-btn', { active: activeTab === tab }]" @click="activeTab = tab">
        {{ $t(`clip.${tab}`) }}
      </button>
    </div>

    <div class="tab-content" v-if="activeTab !== 'history'">
      <form @submit.prevent="submitClip" class="clip-form">
        <div class="form-group" v-if="activeTab === 'text'">
          <label>{{ $t('clip.contentLabel') }}</label>
          <textarea v-model="formData.content" class="input-field" rows="6" required></textarea>
        </div>

        <div class="form-group" v-if="activeTab === 'link'">
          <label>{{ $t('clip.linkUrl') }}</label>
          <input v-model="formData.content" type="url" class="input-field" required />
        </div>

        <div class="form-group" v-if="activeTab === 'file'">
          <label>{{ $t('clip.selectFile') }}</label>
          <div class="file-drop-zone" @dragover.prevent="dragover = true" @dragleave.prevent="dragover = false"
            @drop.prevent="handleDrop" :class="{ dragover }">
            <input type="file" ref="fileInput" @change="handleFileSelect" class="hidden-file-input" />
            <div class="drop-content" @click="$refs.fileInput.click()">
              <p v-if="!selectedFile">Drag & Drop or Click to select</p>
              <p v-else class="file-selected">{{ selectedFile.name }} ({{ formatSize(selectedFile.size) }})</p>
            </div>
          </div>
        </div>

        <div class="options-row">
          <div class="form-group">
            <label>{{ $t('clip.countLabel') }}</label>
            <input v-model="formData.count" type="number" min="1" class="input-field" />
          </div>
          <div class="form-group">
            <label>{{ $t('clip.expireLabel') }}</label>
            <div class="input-with-unit">
              <input v-model="formData.expire" type="number" min="1" class="input-field" />
              <select v-model="formData.expireUnit" class="unit-select">
                <option value="1">{{ $t('clip.units.seconds') }}</option>
                <option value="60">{{ $t('clip.units.minutes') }}</option>
                <option value="3600">{{ $t('clip.units.hours') }}</option>
                <option value="86400">{{ $t('clip.units.days') }}</option>
                <option value="604800">{{ $t('clip.units.weeks') }}</option>
                <option value="31536000">{{ $t('clip.units.years') }}</option>
              </select>
            </div>
          </div>
        </div>

        <button type="submit" class="btn submit-btn" :disabled="loading">
          {{ loading ? '...' : (activeTab === 'file' ? $t('clip.upload') : $t('clip.create')) }}
        </button>
      </form>

      <div v-if="pendingRedirect" class="result-box redirect-box-confirm">
        <p>{{ $t('clip.redirectMsg', { domain: getDomain(route.query.redirect) }) }}</p>
        <div class="redirect-actions">
          <button @click="doRedirect" class="btn">{{ $t('clip.redirectBtn') }}</button>
          <button @click="pendingRedirect = null" class="btn secondary-btn">{{ $t('clip.stayBtn') }}</button>
        </div>
      </div>

      <div v-if="resultCode" class="result-box">
        <p>{{ $t('clip.success') }} <a :href="`/clip/${resultCode}`" target="_blank">{{ resultCode }}</a></p>
        <button @click="copyCode(resultCode)" class="btn copy-btn">{{ $t('clip.copy') }}</button>
      </div>
      <div v-if="errorMsg" class="error-box">
        <p>{{ $t('clip.error') }}{{ errorMsg }}</p>
      </div>
    </div>

    <div class="tab-content history-tab" v-else>
      <div class="history-header">
        <h3>{{ $t('clip.history') }}</h3>
        <button @click="clearHistory" class="btn danger-btn" v-if="history.length">{{ $t('clip.clearHistory')
        }}</button>
      </div>
      <ul class="history-list" v-if="history.length">
        <li v-for="item in history" :key="item.code" class="history-item">
          <span class="history-icon">{{ getIcon(item.type) }}</span>
          <a :href="`/clip/${item.code}`" target="_blank" class="history-link">{{ item.code }}</a>
          <span class="history-type">({{ $t(`clip.${item.type}`) }})</span>
          <span class="history-filename" v-if="item.filename">- {{ item.filename }}</span>
        </li>
      </ul>
      <p v-else class="empty-state">No history yet.</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const { t } = useI18n()

const activeTab = ref('text')
const formData = ref({
  content: '',
  count: 1000000000,
  expire: 10,
  expireUnit: '31536000'
})
const selectedFile = ref(null)
const dragover = ref(false)
const loading = ref(false)
const resultCode = ref('')
const errorMsg = ref('')
const pendingRedirect = ref(null)
const history = ref([])
const fileInput = ref(null)

onMounted(() => {
  history.value = JSON.parse(localStorage.getItem('clipHistory') || '[]')
})

const formatSize = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const handleFileSelect = (e) => {
  selectedFile.value = e.target.files[0]
}

const handleDrop = (e) => {
  dragover.value = false
  if (e.dataTransfer.files.length) {
    selectedFile.value = e.dataTransfer.files[0]
  }
}

const submitClip = async () => {
  loading.value = true
  resultCode.value = ''
  errorMsg.value = ''

  try {
    const fd = new FormData()
    fd.append('count', formData.value.count)
    fd.append('expire', formData.value.expire * parseInt(formData.value.expireUnit))

    let url = '/clip/create'

    if (activeTab.value === 'file') {
      if (!selectedFile.value) {
        errorMsg.value = 'Please select a file'
        loading.value = false
        return
      }
      fd.append('file', selectedFile.value)
      url = '/clip/upload'
    } else {
      fd.append('content', formData.value.content)
      fd.append('link', activeTab.value === 'link' ? 'yes' : 'no')
    }

    const res = await fetch(url, {
      method: 'POST',
      body: fd
    })

    const data = await res.json()
    if (res.ok && data.code) {
      resultCode.value = data.code
      addToHistory(data.code, activeTab.value, selectedFile.value?.name)

      // Handle redirect
      const redirectUrl = route.query.redirect
      if (redirectUrl) {
        const url = new URL(String(redirectUrl))
        url.searchParams.set('filecode', data.code)
        pendingRedirect.value = url.toString()
      }

      formData.value.content = ''
      selectedFile.value = null
    } else {
      errorMsg.value = data.error || 'Failed to create clip'
    }
  } catch (err) {
    errorMsg.value = err.message
  } finally {
    loading.value = false
  }
}

const getDomain = (url) => {
  try {
    return new URL(String(url)).hostname
  } catch (e) {
    return String(url)
  }
}

const doRedirect = () => {
  if (pendingRedirect.value) {
    window.location.href = pendingRedirect.value
  }
}

const addToHistory = (code, type, filename = '') => {
  const entry = { code, type, filename }
  history.value.unshift(entry)
  if (history.value.length > 10) history.value.pop()
  localStorage.setItem('clipHistory', JSON.stringify(history.value))
}

const clearHistory = () => {
  history.value = []
  localStorage.removeItem('clipHistory')
}

const copyCode = async (code) => {
  try {
    await navigator.clipboard.writeText(code)
    alert('Copied!')
  } catch (e) {
    alert('Failed to copy')
  }
}

const getIcon = (type) => {
  if (type === 'file') return '📁'
  if (type === 'link') return '🔗'
  return '📝'
}
</script>

<style scoped>
.header-row {
  margin-bottom: 1rem;
}

.back-link {
  color: var(--muted);
  text-decoration: none;
  font-size: 0.875rem;
  transition: color 0.2s;
}

.back-link:hover {
  color: var(--primary);
}

.info-banner {
  background: rgba(14, 165, 233, 0.1);
  border: 1px solid rgba(14, 165, 233, 0.3);
  color: var(--accent);
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
}

.clip-view {
  animation: slideUp 0.4s ease;
}

.tabs {
  display: flex;
  border-bottom: 1px solid var(--border);
  margin-bottom: 1.5rem;
  overflow-x: auto;
}

.tab-btn {
  background: none;
  border: none;
  padding: 1rem 1.5rem;
  color: var(--muted);
  font-weight: 600;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
  white-space: nowrap;
}

.tab-btn:hover {
  color: var(--fg);
}

.tab-btn.active {
  color: var(--primary);
  border-bottom-color: var(--primary);
}

.clip-form {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
}

.form-group label {
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--muted);
}

.input-with-unit {
  display: flex;
  gap: 0.5rem;
}

.unit-select {
  background: rgba(0, 0, 0, 0.2);
  color: var(--fg);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.5rem;
  outline: none;
}

.light-mode .unit-select {
  background: rgba(255, 255, 255, 0.5);
}

textarea.input-field {
  resize: vertical;
}

.options-row {
  display: flex;
  gap: 1rem;
}

.submit-btn {
  margin-top: 1rem;
  width: 100%;
}

.file-drop-zone {
  border: 2px dashed var(--border);
  border-radius: 0.5rem;
  padding: 3rem 1rem;
  text-align: center;
  transition: all 0.2s;
  background: rgba(0, 0, 0, 0.1);
  cursor: pointer;
}

.file-drop-zone.dragover {
  border-color: var(--primary);
  background: rgba(34, 197, 94, 0.1);
}

.hidden-file-input {
  display: none;
}

.file-selected {
  color: var(--primary);
  font-weight: bold;
}

.result-box {
  margin-top: 1.5rem;
  padding: 1rem;
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.3);
  border-radius: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.result-box a {
  color: var(--primary);
  font-weight: bold;
  text-decoration: none;
  font-size: 1.2rem;
}

.redirect-box-confirm {
  background: rgba(14, 165, 233, 0.1);
  border-color: rgba(14, 165, 233, 0.3);
  flex-direction: column;
  align-items: flex-start;
  gap: 1rem;
}

.redirect-actions {
  display: flex;
  gap: 1rem;
  width: 100%;
}

.secondary-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid var(--border);
  color: var(--fg);
}

.secondary-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

.error-box {
  margin-top: 1.5rem;
  padding: 1rem;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 0.5rem;
  color: #ef4444;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.danger-btn {
  background: transparent;
  color: #ef4444;
  border: 1px solid #ef4444;
  padding: 0.5rem 1rem;
}

.danger-btn:hover {
  background: #ef4444;
  color: white;
}

.history-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.history-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 0.5rem;
  border: 1px solid var(--border);
}

.light-mode .history-item {
  background: rgba(255, 255, 255, 0.5);
}

.history-link {
  color: var(--accent);
  font-weight: bold;
  text-decoration: none;
}

.history-link:hover {
  text-decoration: underline;
}

.history-type {
  color: var(--muted);
  font-size: 0.875rem;
}

.empty-state {
  text-align: center;
  color: var(--muted);
  padding: 2rem;
}
</style>
