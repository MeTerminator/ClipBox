<template>
  <Teleport to="body">
    <Transition name="detail-modal">
      <div
        v-if="open && item"
        class="fixed inset-0 z-[100] flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        :aria-label="t('clip.fileDetails')"
        @click.self="close"
      >
        <section class="detail-modal-panel w-full max-w-[820px] overflow-hidden border border-white/20 bg-[#090a0b] text-white shadow-2xl">
          <header class="flex items-center justify-between border-b border-white/10 px-5 py-4 sm:px-6">
            <h2 class="text-lg font-bold tracking-tight sm:text-xl">{{ item.type === 'file' ? t('clip.fileDetails') : t('clip.pickupDetails') }}</h2>
            <button
              type="button"
              class="p-1.5 text-white/55 transition hover:bg-white/10 hover:text-white"
              :aria-label="t('clip.close')"
              @click="close"
            >
              <XIcon class="h-5 w-5" />
            </button>
          </header>

          <div class="space-y-6 p-5 sm:p-6">
            <div class="flex items-center gap-4 border border-white/10 bg-white/[0.045] p-4 sm:p-5">
              <div class="flex h-14 w-14 shrink-0 items-center justify-center border border-white/15 bg-white/[0.08] text-white/85">
                <FileIcon v-if="item.type === 'file'" class="h-7 w-7" />
                <LinkIcon v-else-if="item.type === 'link'" class="h-7 w-7" />
                <FileTextIcon v-else class="h-7 w-7" />
              </div>
              <div class="min-w-0">
                <p class="truncate text-base font-semibold sm:text-lg">{{ item.filename || typeLabel }}</p>
                <p v-if="item.expiresAt || item.remainingCount !== undefined" class="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-white/60">
                  <span v-if="item.expiresAt">{{ t('clip.expiresAt') }} {{ formatDate(item.expiresAt) }}</span>
                  <span v-if="item.remainingCount !== undefined">{{ t('home.remainingCount') }} {{ item.remainingCount }}</span>
                </p>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_minmax(280px,0.9fr)] sm:grid-rows-[auto_auto]">
              <div class="border border-white/20 bg-[#e7e7e9] p-5 text-[#111214]">
                <div class="flex items-center justify-between gap-3">
                  <span class="text-sm font-semibold">{{ t('clip.pickupCode') }}</span>
                  <button type="button" class="p-1.5 transition hover:bg-black/10" :aria-label="t('clip.copyCode')" @click="copyCode">
                    <CopyIcon class="h-5 w-5" />
                  </button>
                </div>
                <p class="mt-5 text-center font-mono text-4xl font-bold tracking-[0.18em]">{{ item.code }}</p>
              </div>

              <div class="grid min-w-0 grid-cols-2 gap-4">
                <button type="button" class="flex min-w-0 items-center gap-3 border border-white/10 bg-white/[0.045] p-4 text-left transition hover:bg-white/[0.08]" @click="copyWget">
                  <TerminalIcon class="h-5 w-5 shrink-0 text-white/65" />
                  <span class="min-w-0">
                    <span class="block whitespace-nowrap text-sm font-semibold">wget {{ t('clip.download') }}</span>
                    <span class="mt-1 block truncate whitespace-nowrap text-xs text-white/55">{{ t('clip.copyWget') }}</span>
                  </span>
                  <CopyIcon class="ml-auto h-4 w-4 shrink-0 text-white/55" />
                </button>

                <button type="button" class="flex min-w-0 items-center gap-3 border border-white/10 bg-white/[0.045] p-4 text-left transition hover:bg-white/[0.08]" @click="copyCurl">
                  <TerminalIcon class="h-5 w-5 shrink-0 text-white/65" />
                  <span class="min-w-0">
                    <span class="block whitespace-nowrap text-sm font-semibold">curl {{ t('clip.download') }}</span>
                    <span class="mt-1 block truncate whitespace-nowrap text-xs text-white/55">{{ t('clip.copyCurl') }}</span>
                  </span>
                  <CopyIcon class="ml-auto h-4 w-4 shrink-0 text-white/55" />
                </button>
              </div>

              <div class="flex min-h-[224px] flex-col items-center justify-center border border-white/10 bg-white/[0.045] p-5 sm:col-start-2 sm:row-start-1 sm:row-span-2">
                <div class="flex h-[174px] w-[174px] items-center justify-center border border-black/10 bg-white p-3">
                  <img v-if="qrDataUrl" :src="qrDataUrl" :alt="t('clip.qrAlt')" class="h-full w-full" />
                  <Loader2Icon v-else class="h-7 w-7 animate-spin text-black/50" />
                </div>
                <p class="mt-4 text-center text-sm text-white/60">{{ t('clip.scanToPickup') }}</p>
              </div>
            </div>
          </div>

          <footer class="border-t border-white/10 p-5 sm:p-6">
            <button type="button" class="h-12 w-full border border-white/20 bg-[#e7e7e9] px-4 text-sm font-bold text-[#111214] transition hover:bg-white" @click="copyLink">
              {{ copied ? t('clip.copied') : t('clip.copyPickupLink') }}
            </button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import QRCode from 'qrcode'
import { useI18n } from 'vue-i18n'
import {
  CopyIcon,
  FileIcon,
  FileTextIcon,
  LinkIcon,
  Loader2Icon,
  TerminalIcon,
  XIcon,
} from 'lucide-vue-next'

const props = defineProps({
  open: { type: Boolean, default: false },
  item: { type: Object, default: null },
})

const emit = defineEmits(['close', 'copied'])
const { t, locale } = useI18n()
const qrDataUrl = ref('')
const copied = ref(false)

const pickupLink = computed(() => {
  if (!props.item?.code) return ''
  return `${window.location.origin}/clip/${props.item.code}`
})

const typeLabel = computed(() => t(`clip.${props.item?.type || 'file'}`))

watch(() => [props.open, props.item?.code], async ([open]) => {
  copied.value = false
  qrDataUrl.value = ''
  if (!open || !pickupLink.value) return
  try {
    qrDataUrl.value = await QRCode.toDataURL(pickupLink.value, {
      width: 180,
      margin: 0,
      errorCorrectionLevel: 'M',
      color: { dark: '#08090a', light: '#ffffff' },
    })
  } catch {
    qrDataUrl.value = ''
  }
}, { immediate: true })

const close = () => emit('close')

const formatDate = (value) => {
  if (!value) return ''
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

const writeClipboard = async (value, markCopied = false) => {
  try {
    await navigator.clipboard.writeText(value)
    if (markCopied) copied.value = true
    emit('copied')
  } catch {
    // Keep the dialog open when clipboard permissions are unavailable.
  }
}

const copyCode = () => writeClipboard(props.item?.code || '')
const copyLink = () => writeClipboard(pickupLink.value, true)
const copyWget = () => writeClipboard(`wget -O "${props.item?.filename || 'download'}" "${pickupLink.value}"`)
const copyCurl = () => writeClipboard(`curl -L -o "${props.item?.filename || 'download'}" "${pickupLink.value}"`)
</script>

<style scoped>
.detail-modal-enter-active,
.detail-modal-leave-active {
  transition: opacity 180ms ease;
}

.detail-modal-enter-active .detail-modal-panel,
.detail-modal-leave-active .detail-modal-panel {
  transition: transform 180ms ease, opacity 180ms ease;
}

.detail-modal-enter-from,
.detail-modal-leave-to {
  opacity: 0;
}

.detail-modal-enter-from .detail-modal-panel,
.detail-modal-leave-to .detail-modal-panel {
  opacity: 0;
  transform: translateY(10px) scale(0.98);
}
</style>
