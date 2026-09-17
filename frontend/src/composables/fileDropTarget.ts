import { shallowRef } from "vue";

export type RoomFileDropHandler = (file: File) => Promise<void>;

export const roomFileDropHandler = shallowRef<RoomFileDropHandler | null>(null);

export function setRoomFileDropHandler(handler: RoomFileDropHandler | null) {
  roomFileDropHandler.value = handler;
}
