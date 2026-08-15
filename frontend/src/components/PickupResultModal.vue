<template>
  <Teleport to="body">
    <Transition name="pickup-modal">
      <div
        v-if="open && result"
        class="fixed inset-0 z-[100] flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        :aria-label="t('home.resultTitle')"
        @click.self="close"
      >
        <section class="pickup-modal-panel w-full max-w-[620px] overflow-hidden border border-white/20 bg-[#090a0b] text-white shadow-2xl">
          <header class="flex items-center justify-between border-b border-white/10 px-5 py-4 sm:px-6">
            <div class="flex items-center gap-3">
              <LinkIcon v-if="result.type === 'link'" class="h-5 w-5 text-white/70" />
              <FileIcon v-else-if="result.type === 'file'" class="h-5 w-5 text-white/70" />
              <FileTextIcon v-else class="h-5 w-5 text-white/70" />
              <h2 class="text-lg font-bold tracking-tight">{{ resultTitle }}</h2>
            </div>
            <button type="button" class="p-1.5 text-white/55 transition hover:bg-white/10 hover:text-white" :aria-label="t('clip.close')" @click="close">
              <XIcon class="h-5 w-5" />
            </button>
          </header>

          <div class="space-y-5 p-5 sm:p-6">
            <div class="grid grid-cols-1 border border-white/15 text-sm sm:grid-cols-2">
              <div class="border-b border-white/15 p-4 sm:border-b-0 sm:border-r">
                <p class="text-xs text-white/50">{{ t('clip.expiresAt') }}</p>
                <p class="mt-1 font-medium">{{ formatDate(result.expires_at) }}</p>
              </div>
              <div class="p-4">
                <p class="text-xs text-white/50">{{ t('home.remainingCount') }}</p>
                <p class="mt-1 font-medium">{{ result.remaining_count }}</p>
              </div>
            </div>

            <template v-if="result.type === 'link'">
              <div class="border border-white/15 bg-white/[0.045] p-4">
                <p class="text-xs text-white/50">{{ t('home.linkAddress') }}</p>
                <p class="mt-2 break-all font-mono text-sm leading-6">{{ result.content }}</p>
              </div>
              <p class="text-sm text-white/65">{{ t('home.confirmRedirect') }}</p>
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <button type="button" class="h-12 border border-white/20 bg-white px-4 font-bold text-black transition hover:bg-white/85" @click="openLink">
                  {{ t('home.openLink') }}
                </button>
                <button type="button" class="h-12 border border-white/20 bg-transparent px-4 font-bold text-white transition hover:bg-white/10" @click="close">
                  {{ t('clip.stayBtn') }}
                </button>
              </div>
            </template>

            <template v-else-if="result.type === 'text/plain'">
              <div class="max-h-[320px] overflow-auto whitespace-pre-wrap break-words border border-white/15 bg-white/[0.045] p-4 font-mono text-sm leading-6">{{ result.content }}</div>
              <button type="button" class="flex h-12 w-full items-center justify-center gap-2 border border-white/20 bg-white px-4 font-bold text-black transition hover:bg-white/85" @click="copyText">
                <CopyIcon class="h-4 w-4" />
                {{ t('home.copyText') }}
              </button>
            </template>

            <template v-else>
              <div class="flex items-center gap-4 border border-white/15 bg-white/[0.045] p-4">
                <div class="flex h-12 w-12 shrink-0 items-center justify-center border border-white/15 bg-white/[0.06]">
                  <FileIcon class="h-6 w-6 text-white/75" />
                </div>
                <div class="min-w-0">
                  <p class="truncate font-semibold">{{ result.filename }}</p>
                  <p class="mt-1 text-xs text-white/50">{{ formatSize(result.size) }}</p>
                </div>
              </div>
              <a :href="result.download_url" class="flex h-12 w-full items-center justify-center gap-2 border border-white/20 bg-white px-4 font-bold text-black transition hover:bg-white/85" download>
                <DownloadIcon class="h-4 w-4" />
                {{ t('home.downloadFile') }}
              </a>
            </template>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CopyIcon, DownloadIcon, FileIcon, FileTextIcon, LinkIcon, XIcon } from 'lucide-vue-next'

const props = defineProps({
  open: { type: Boolean, default: false },
  result: { type: Object, default: null },
})

const emit = defineEmits(['close', 'copied'])
const { t, locale } = useI18n()

const resultTitle = computed(() => {
  if (props.result?.type === 'link') return t('home.linkResult')
  if (props.result?.type === 'file') return t('home.fileResult')
  return t('home.textResult')
})

const close = () => emit('close')

const openLink = () => {
  window.location.assign(props.result?.content || '/')
}

const copyText = async () => {
  try {
    await navigator.clipboard.writeText(props.result?.content || '')
    emit('copied')
  } catch {
    // Leave the content visible when clipboard access is unavailable.
  }
}

const formatDate = (value) => {
  if (!value) return '-'
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

const formatSize = (bytes = 0) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** index)).toFixed(index ? 2 : 0)} ${units[index]}`
}
</script>

<style scoped>
.pickup-modal-enter-active,
.pickup-modal-leave-active {
  transition: opacity 180ms ease;
}

.pickup-modal-enter-active .pickup-modal-panel,
.pickup-modal-leave-active .pickup-modal-panel {
  transition: transform 180ms ease, opacity 180ms ease;
}

.pickup-modal-enter-from,
.pickup-modal-leave-to {
  opacity: 0;
}

.pickup-modal-enter-from .pickup-modal-panel,
.pickup-modal-leave-to .pickup-modal-panel {
  opacity: 0;
  transform: translateY(10px) scale(0.98);
}
</style>
