import { shallowRef } from "vue";

export type RoomFileDropHandler = (file: File) => Promise<void>;
export type RoomFileCodeHandler = (code: string, roomId: string) => Promise<void>;

export const roomFileDropHandler = shallowRef<RoomFileDropHandler | null>(null);
export const roomFileCodeHandler = shallowRef<RoomFileCodeHandler | null>(null);

export function setRoomFileDropHandler(handler: RoomFileDropHandler | null) {
  roomFileDropHandler.value = handler;
}

export function setRoomFileCodeHandler(handler: RoomFileCodeHandler | null) {
  roomFileCodeHandler.value = handler;
}
