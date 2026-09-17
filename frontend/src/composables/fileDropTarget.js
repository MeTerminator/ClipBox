import { shallowRef } from "vue";

export const roomFileDropHandler = shallowRef(null);

export function setRoomFileDropHandler(handler) {
  roomFileDropHandler.value = handler;
}
