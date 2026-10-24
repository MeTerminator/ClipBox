import { shallowRef } from "vue";

export type RoomFileDropHandler = (file: File) => Promise<void>;
export type RoomFileCodeHandler = (code: string, roomId: string) => Promise<void>;

// Keep the dropped file until the create page mounts and selects it for review.
export const pendingShareFile = shallowRef<File | null>(null);

export const roomFileDropHandler = shallowRef<RoomFileDropHandler | null>(null);
export const roomFileCodeHandler = shallowRef<RoomFileCodeHandler | null>(null);

export function setRoomFileDropHandler(handler: RoomFileDropHandler | null) {
  roomFileDropHandler.value = handler;
}

export function setRoomFileCodeHandler(handler: RoomFileCodeHandler | null) {
  roomFileCodeHandler.value = handler;
}
