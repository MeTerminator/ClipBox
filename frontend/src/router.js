import { createRouter, createWebHistory } from "vue-router";
import Home from "./views/Home.vue";
import Clip from "./views/Clip.vue";
import Rooms from "./views/Rooms.vue";

const routes = [
  { path: "/", name: "home", component: Home },
  { path: "/create", name: "create", component: Clip },
  { path: "/clip", redirect: "/create" },
  { path: "/rooms/:roomID?", name: "rooms", component: Rooms },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});
