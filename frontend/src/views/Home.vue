<template>
  <div class="flex flex-col gap-8 animate-in fade-in slide-in-from-bottom-5 duration-500">
    <!-- Hero / Pickup Section -->
    <Card class="border border-border bg-card shadow-none text-center py-12 px-6">
      <CardHeader class="pb-2">
        <CardTitle class="text-4xl font-extrabold tracking-tight text-foreground pb-1">
          {{ $t('home.title') }}
        </CardTitle>
        <CardDescription class="text-muted-foreground text-base max-w-md mx-auto pt-2">
          {{ $t('home.desc') }}
        </CardDescription>
      </CardHeader>
      
      <CardContent class="pt-6">
        <form @submit.prevent="pickup" class="flex flex-col sm:flex-row gap-3 max-w-md mx-auto justify-center">
          <Input 
            v-model="code" 
            type="text" 
            maxlength="5" 
            pattern="\d{5}" 
            :placeholder="$t('home.pickupCode')"
            class="text-center text-lg h-12 tracking-widest font-mono bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground" 
            required 
          />
          <Button type="submit" class="h-12 px-6 font-semibold shadow-none border border-foreground bg-foreground text-background hover:bg-background hover:text-foreground transition-colors duration-200">
            {{ $t('home.pickupBtn') }}
          </Button>
        </form>
      </CardContent>

      <CardFooter class="justify-center pt-2">
        <p class="text-xs text-muted-foreground max-w-sm">
          {{ $t('home.tips') }}
        </p>
      </CardFooter>
    </Card>

    <!-- Quick Actions / Features Grid -->
    <div class="grid grid-cols-1 gap-6">
      <router-link to="/create" class="block group transition-all duration-200">
        <Card class="border border-border bg-transparent group-hover:bg-foreground group-hover:border-foreground shadow-none transition-all duration-200">
          <CardHeader>
            <CardTitle class="text-xl font-bold flex items-center gap-2 group-hover:text-background transition-colors">
              {{ $t('home.clipTitle') }}
            </CardTitle>
            <CardDescription class="text-muted-foreground mt-1 group-hover:text-background/80 transition-colors">
              {{ $t('home.clipDesc') }}
            </CardDescription>
          </CardHeader>
        </Card>
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { toast } from 'vue-sonner'

const code = ref('')

const pickup = () => {
  if (code.value && /^\d{5}$/.test(code.value)) {
    window.open(`/clip/${code.value}`, '_blank')
  } else {
    toast.error("Please enter a valid 5-digit code")
  }
}
</script>
