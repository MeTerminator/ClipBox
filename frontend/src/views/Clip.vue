<template>
  <Card class="border border-border bg-card shadow-none p-6 animate-in fade-in duration-200">
    <!-- Header Back Link -->
    <div class="mb-6">
      <router-link to="/" class="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors">
        <ArrowLeftIcon class="h-4 w-4" />
        {{ $t('clip.backToHome') || 'Back' }}
      </router-link>
    </div>

    <!-- Redirect notice banner -->
    <div v-if="route.query.redirect" class="flex items-start gap-2.5 p-3 border border-border bg-transparent text-foreground rounded-lg text-sm mb-6">
      <InfoIcon class="h-4 w-4 mt-0.5 shrink-0" />
      <span>{{ $t('clip.redirectNotice', { domain: getDomain(route.query.redirect) }) }}</span>
    </div>

    <!-- Tabs Navigation -->
    <Tabs v-model="activeTab" class="w-full">
      <TabsList class="grid w-full grid-cols-4 mb-6">
        <TabsTrigger value="text">{{ $t('clip.text') }}</TabsTrigger>
        <TabsTrigger value="link">{{ $t('clip.link') }}</TabsTrigger>
        <TabsTrigger value="file">{{ $t('clip.file') }}</TabsTrigger>
        <TabsTrigger value="history">{{ $t('clip.history') }}</TabsTrigger>
      </TabsList>
    </Tabs>

    <!-- Form Section -->
    <div v-if="activeTab !== 'history'">
      <form @submit.prevent="submitClip" class="space-y-6">
        <!-- Text Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'text'">
          <Label class="text-sm font-semibold text-muted-foreground">{{ $t('clip.contentLabel') }}</Label>
          <Textarea 
            v-model="formData.content" 
            class="min-h-[150px] bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground" 
            required 
          />
        </div>

        <!-- Link Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'link'">
          <Label class="text-sm font-semibold text-muted-foreground">{{ $t('clip.linkUrl') }}</Label>
          <Input 
            v-model="formData.content" 
            type="url" 
            class="bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground" 
            required 
          />
        </div>

        <!-- File Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'file'">
          <Label class="text-sm font-semibold text-muted-foreground">{{ $t('clip.selectFile') }}</Label>
          <div 
            @dragover.prevent="dragover = true" 
            @dragleave.prevent="dragover = false"
            @drop.prevent="handleDrop" 
            :class="[
              'rounded-lg p-8 text-center cursor-pointer transition-colors duration-200 flex flex-col items-center justify-center min-h-[150px]',
              dragover ? 'border-foreground border-solid' : 'border-border hover:border-foreground border-dashed border'
            ]"
            @click="fileInput.click()"
          >
            <input type="file" ref="fileInput" @change="handleFileSelect" class="hidden" />
            <div class="space-y-2">
              <div class="flex justify-center text-muted-foreground">
                <UploadCloudIcon class="h-10 w-10 stroke-1" />
              </div>
              <div class="text-sm">
                <p v-if="!selectedFile" class="font-medium text-foreground">
                  Drag & Drop or Click to select
                </p>
                <p v-else class="font-bold text-foreground">
                  {{ selectedFile.name }} ({{ formatSize(selectedFile.size) }})
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Access Options Row -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="flex flex-col gap-2">
            <Label class="text-sm font-semibold text-muted-foreground">{{ $t('clip.countLabel') }}</Label>
            <Input 
              v-model="formData.count" 
              type="number" 
              min="1" 
              class="bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground" 
            />
          </div>
          <div class="flex flex-col gap-2">
            <Label class="text-sm font-semibold text-muted-foreground">{{ $t('clip.expireLabel') }}</Label>
            <div class="flex gap-2">
              <Input 
                v-model="formData.expire" 
                type="number" 
                min="1" 
                class="flex-1 bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground" 
              />
              <Select v-model="formData.expireUnit">
                <SelectTrigger class="w-[110px] bg-transparent border-border focus:ring-1 focus:ring-foreground">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">{{ $t('clip.units.seconds') }}</SelectItem>
                  <SelectItem value="60">{{ $t('clip.units.minutes') }}</SelectItem>
                  <SelectItem value="3600">{{ $t('clip.units.hours') }}</SelectItem>
                  <SelectItem value="86400">{{ $t('clip.units.days') }}</SelectItem>
                  <SelectItem value="604800">{{ $t('clip.units.weeks') }}</SelectItem>
                  <SelectItem value="31536000">{{ $t('clip.units.years') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        <!-- Submit Button -->
        <Button 
          type="submit" 
          class="w-full font-semibold shadow-none border border-foreground bg-foreground text-background hover:bg-background hover:text-foreground py-6 transition-colors"
          :disabled="loading"
        >
          {{ loading ? '...' : (activeTab === 'file' ? $t('clip.upload') : $t('clip.create')) }}
        </Button>
      </form>

      <!-- Redirect Confirmation Box -->
      <div v-if="pendingRedirect" class="mt-6 p-4 border border-border bg-transparent rounded-lg space-y-3">
        <p class="text-muted-foreground text-sm">{{ $t('clip.redirectMsg', { domain: getDomain(route.query.redirect) }) }}</p>
        <div class="flex gap-3">
          <Button @click="doRedirect" size="sm" class="text-xs font-semibold">
            {{ $t('clip.redirectBtn') }}
          </Button>
          <Button @click="pendingRedirect = null" size="sm" variant="outline" class="text-xs border-border bg-transparent hover:bg-foreground hover:text-background">
            {{ $t('clip.stayBtn') }}
          </Button>
        </div>
      </div>

      <!-- Result Code Box -->
      <div v-if="resultCode" class="mt-6 p-4 border border-border bg-transparent rounded-lg flex items-center justify-between gap-4">
        <span class="text-sm font-semibold text-foreground flex items-center gap-1">
          {{ $t('clip.success') }}
          <a :href="`/clip/${resultCode}`" target="_blank" class="underline hover:text-foreground/80 font-mono font-bold">
            {{ resultCode }}
          </a>
        </span>
        <Button 
          @click="copyCode(resultCode)" 
          size="sm" 
          variant="outline" 
          class="gap-1.5 border-border bg-transparent hover:bg-foreground hover:text-background"
        >
          <CopyIcon class="h-3.5 w-3.5" />
          {{ $t('clip.copy') }}
        </Button>
      </div>

      <!-- Error Message Box -->
      <div v-if="errorMsg" class="mt-6 p-4 border border-destructive bg-transparent rounded-lg text-sm text-destructive">
        {{ $t('clip.error') }}{{ errorMsg }}
      </div>
    </div>

    <!-- History Tab Section -->
    <div v-else class="space-y-4">
      <div class="flex items-center justify-between pb-2">
        <h3 class="text-lg font-bold text-foreground">{{ $t('clip.history') }}</h3>
        <Button 
          v-if="history.length" 
          @click="clearHistory" 
          variant="outline" 
          size="sm" 
          class="border-border bg-transparent hover:bg-destructive hover:text-destructive-foreground hover:border-destructive gap-1.5"
        >
          <Trash2Icon class="h-3.5 w-3.5" />
          {{ $t('clip.clearHistory') }}
        </Button>
      </div>

      <ul v-if="history.length" class="space-y-2">
        <li 
          v-for="item in history" 
          :key="item.code" 
          class="flex items-center justify-between p-3 rounded-lg border border-border bg-transparent hover:border-foreground transition-colors"
        >
          <div class="flex items-center gap-3 overflow-hidden">
            <span class="text-muted-foreground shrink-0">
              <component :is="getIconComponent(item.type)" class="h-4 w-4" />
            </span>
            <a :href="`/clip/${item.code}`" target="_blank" class="font-bold font-mono text-foreground hover:underline hover:text-foreground/80 flex items-center gap-1 shrink-0">
              {{ item.code }}
              <ExternalLinkIcon class="h-3 w-3 text-muted-foreground" />
            </a>
            <span class="text-xs text-muted-foreground shrink-0">({{ $t(`clip.${item.type}`) }})</span>
            <span v-if="item.filename" class="text-xs text-muted-foreground font-medium truncate max-w-[150px] sm:max-w-[300px]">
              - {{ item.filename }}
            </span>
          </div>
        </li>
      </ul>
      <div v-else class="text-center py-12 border border-dashed rounded-lg border-border/60 text-muted-foreground text-sm">
        No history yet.
      </div>
    </div>
  </Card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Card } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { toast } from 'vue-sonner'
import { 
  ArrowLeftIcon, 
  UploadCloudIcon, 
  CopyIcon, 
  Trash2Icon, 
  FileTextIcon, 
  LinkIcon, 
  FileIcon, 
  ExternalLinkIcon,
  InfoIcon
} from 'lucide-vue-next'

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
      toast.success(t('clip.success') + data.code)

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
      toast.error(errorMsg.value)
    }
  } catch (err) {
    errorMsg.value = err.message
    toast.error(errorMsg.value)
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
  toast.success('History cleared')
}

const copyCode = async (code) => {
  try {
    await navigator.clipboard.writeText(code)
    toast.success(t('clip.copied') || 'Copied!')
  } catch (e) {
    toast.error('Failed to copy')
  }
}

const getIconComponent = (type) => {
  if (type === 'file') return FileIcon
  if (type === 'link') return LinkIcon
  return FileTextIcon
}
</script>
