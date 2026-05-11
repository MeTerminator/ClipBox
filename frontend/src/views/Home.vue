<template>
  <div class="home">
    <div class="glass-card hero-section">
      <h1 class="title">{{ $t('home.title') }}</h1>
      <p class="subtitle">{{ $t('home.desc') }}</p>

      <form @submit.prevent="pickup" class="pickup-form">
        <input v-model="code" type="text" maxlength="5" pattern="\d{5}" :placeholder="$t('home.pickupCode')"
          class="input-field code-input" required />
        <button type="submit" class="btn pickup-btn">{{ $t('home.pickupBtn') }}</button>
      </form>
      <p class="tips">{{ $t('home.tips') }}</p>
    </div>

    <div class="grid-links">
      <router-link to="/create" class="glass-card nav-card">
        <h3>{{ $t('home.clipTitle') }}</h3>
        <p>{{ $t('home.clipDesc') }}</p>
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const code = ref('')

const pickup = () => {
  if (code.value && /^\d{5}$/.test(code.value)) {
    window.open(`/clip/${code.value}`, '_blank')
  } else {
    alert("Please enter a valid 5-digit code")
  }
}
</script>

<style scoped>
.home {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  animation: slideUp 0.5s ease;
}

.hero-section {
  text-align: center;
  padding: 3rem 2rem;
}

.title {
  font-size: 2.5rem;
  margin-bottom: 1rem;
  background: linear-gradient(to right, var(--primary), var(--accent));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.subtitle {
  color: var(--muted);
  margin-bottom: 2rem;
}

.pickup-form {
  display: flex;
  gap: 1rem;
  max-width: 500px;
  margin: 0 auto 1.5rem;
}

.code-input {
  flex: 1;
  font-size: 1.25rem;
  letter-spacing: 2px;
  text-align: center;
}

.tips {
  font-size: 0.875rem;
  color: var(--muted);
}

.grid-links {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1.5rem;
}

.nav-card {
  text-decoration: none;
  color: var(--fg);
  padding: 1.5rem;
  transition: transform 0.2s, background 0.2s;
  cursor: pointer;
  display: block;
}

.nav-card:hover {
  transform: translateY(-5px);
  background: rgba(255, 255, 255, 0.05);
}

.light-mode .nav-card:hover {
  background: rgba(0, 0, 0, 0.02);
}

.nav-card h3 {
  margin-top: 0;
  margin-bottom: 0.5rem;
}

.nav-card p {
  color: var(--muted);
  font-size: 0.875rem;
  margin: 0;
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
