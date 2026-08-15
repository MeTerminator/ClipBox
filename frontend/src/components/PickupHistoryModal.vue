<template>
  <Teleport to="body">
    <Transition name="history-modal">
      <div
        v-if="open"
        class="fixed inset-0 z-[100] flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        :aria-label="t('home.pickupHistory')"
        @click.self="emit('close')"
      >
        <section class="history-modal-panel w-full max-w-[620px] overflow-hidden border border-white/20 bg-[#090a0b] text-white shadow-2xl">
          <header class="flex items-center justify-between border-b border-white/10 px-5 py-4 sm:px-6">
            <h2 class="text-lg font-bold">{{ t('home.pickupHistory') }}</h2>
            <button type="button" class="p-1.5 text-white/55 transition hover:bg-white/10 hover:text-white" :aria-label="t('clip.close')" @click="emit('close')">
              <XIcon class="h-5 w-5" />
            </button>
          </header>

          <div class="max-h-[60vh] overflow-auto p-5 sm:p-6">
            <ul v-if="records.length" class="space-y-2">
              <li v-for="record in records" :key="`${record.code}-${record.retrievedAt}`">
                <button type="button" class="flex w-full items-center gap-3 border border-white/15 p-4 text-left transition hover:bg-white/[0.07]" @click="emit('select', record)">
                  <FileIcon v-if="record.type === 'file'" class="h-5 w-5 shrink-0 text-white/60" />
                  <LinkIcon v-else-if="record.type === 'link'" class="h-5 w-5 shrink-0 text-white/60" />
                  <FileTextIcon v-else class="h-5 w-5 shrink-0 text-white/60" />
                  <span class="min-w-0 flex-1">
                    <span class="block truncate font-semibold">{{ record.filename || typeLabel(record.type) }}</span>
                    <span class="mt-1 block text-xs text-white/50">{{ record.code }} · {{ formatDate(record.retrievedAt) }}</span>
                  </span>
                  <ChevronRightIcon class="h-4 w-4 shrink-0 text-white/45" />
                </button>
              </li>
            </ul>
            <div v-else class="border border-dashed border-white/20 px-4 py-12 text-center text-sm text-white/50">
              {{ t('home.noPickupHistory') }}
            </div>
          </div>

          <footer v-if="records.length" class="border-t border-white/10 p-5 sm:p-6">
            <button type="button" class="h-11 w-full border border-white/20 bg-transparent px-4 text-sm font-semibold transition hover:bg-white/10" @click="emit('clear')">
              {{ t('home.clearPickupHistory') }}
            </button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { ChevronRightIcon, FileIcon, FileTextIcon, LinkIcon, XIcon } from 'lucide-vue-next'

defineProps({
  open: { type: Boolean, default: false },
  records: { type: Array, default: () => [] },
})

const emit = defineEmits(['close', 'select', 'clear'])
const { t, locale } = useI18n()

const typeLabel = type => {
  if (type === 'file') return t('home.fileResult')
  if (type === 'link') return t('home.linkResult')
  return t('home.textResult')
}

const formatDate = value => new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
  dateStyle: 'medium',
  timeStyle: 'short',
}).format(new Date(value))
</script>

<style scoped>
.history-modal-enter-active,
.history-modal-leave-active {
  transition: opacity 180ms ease;
}

.history-modal-enter-active .history-modal-panel,
.history-modal-leave-active .history-modal-panel {
  transition: transform 180ms ease, opacity 180ms ease;
}

.history-modal-enter-from,
.history-modal-leave-to {
  opacity: 0;
}

.history-modal-enter-from .history-modal-panel,
.history-modal-leave-to .history-modal-panel {
  opacity: 0;
  transform: translateY(10px) scale(0.98);
}
</style>
