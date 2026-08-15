import { createApp } from 'vue'
import 'vue-sonner/style.css'
import './style.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'

createApp(App).use(router).use(i18n).mount('#app')
