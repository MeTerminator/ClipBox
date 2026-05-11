import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Clip from './views/Clip.vue'

const routes = [
  { path: '/', component: Home },
  { path: '/create', component: Clip },
  { path: '/clip', redirect: '/create' },
]

export default createRouter({
  history: createWebHistory(),
  routes,
})
