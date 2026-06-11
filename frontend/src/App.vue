<template>
  <Toaster />
  <header class="sticky top-0 z-50 w-full bg-background border-b border-border">
    <div class="container flex h-14 max-w-screen-md items-center justify-between py-0">
      <router-link to="/" class="flex items-center gap-2 font-bold text-lg text-foreground hover:text-foreground/80 transition-colors">
        <span>ClipBox</span>
      </router-link>
      <div class="flex items-center gap-4">
        <Select v-model="locale" @update:modelValue="saveLang">
          <SelectTrigger class="w-[110px] h-9 border-border bg-transparent focus:ring-1 focus:ring-foreground">
            <SelectValue placeholder="Language" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="en">English</SelectItem>
            <SelectItem value="zh">简体中文</SelectItem>
          </SelectContent>
        </Select>

        <Button variant="outline" size="icon" @click="toggleTheme" class="h-9 w-9 border-border bg-transparent hover:bg-foreground hover:text-background transition-colors">
          <Sun v-if="isLight" class="h-4 w-4" />
          <Moon v-else class="h-4 w-4" />
        </Button>
      </div>
    </div>
  </header>

  <main class="container max-w-screen-md py-8 flex-1">
    <router-view v-slot="{ Component }">
      <transition name="fade" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
  </main>

  <footer class="bg-background border-t border-border py-6 mt-12">
    <div class="container max-w-screen-md flex flex-col items-center justify-between gap-4">
      <p class="text-center text-sm leading-loose text-muted-foreground">
        &copy; {{ new Date().getFullYear() }} 
        <a href="https://github.com/MeTerminator/ClipBox" target="_blank" class="font-medium underline underline-offset-4 hover:text-foreground">
          ClipBox
        </a>
      </p>
    </div>
  </footer>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Toaster } from '@/components/ui/sonner'
import { Button } from '@/components/ui/button'
import { Sun, Moon } from 'lucide-vue-next'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const { locale } = useI18n()
const isLight = ref(false)

onMounted(() => {
  const theme = localStorage.getItem('theme') || 'dark'
  isLight.value = theme === 'light'
  if (theme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
})

const toggleTheme = () => {
  isLight.value = !isLight.value
  if (isLight.value) {
    document.documentElement.classList.remove('dark')
    localStorage.setItem('theme', 'light')
  } else {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'dark')
  }
}

const saveLang = (value) => {
  localStorage.setItem('lang', value)
}
</script>

<style scoped>
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
