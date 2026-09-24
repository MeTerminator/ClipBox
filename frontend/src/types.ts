export type ClipKind = "text" | "text/plain" | "link" | "file";

export interface ClipRecord {
  kind?: "clip";
  code: string;
  type: ClipKind;
  content?: string;
  filename?: string;
  size?: number;
  expiresAt?: string;
  expires_at?: string;
  remainingCount?: number;
  remaining_count?: number;
  maxCount?: number;
  max_count?: number;
  download_url?: string;
  retrievedAt?: string;
  expired?: boolean;
}

export interface RoomMember {
  id: number;
  nickname: string;
  device: string;
  os: string;
  browser: string;
  user_agent?: string;
  is_owner: boolean;
  online: boolean;
  last_seen_at: string;
}

export interface RoomInfo {
  id: string;
  name: string;
  has_password?: boolean;
  members: RoomMember[];
  current_member_id: number;
  created_at: string;
}

export interface RoomSession {
  room: RoomInfo;
  token: string;
}

export interface SavedRoom {
  id: string;
  name: string;
  nickname: string;
  token: string;
  isOwner?: boolean;
  hasPassword?: boolean;
  joinedAt?: string;
}

export interface RoomHistoryRecord {
  kind: "room";
  id: string;
  name: string;
  nickname: string;
  token: string;
  isOwner: boolean;
  hasPassword: boolean;
  joinedAt: string;
}

export type PickupHistoryRecord = ClipRecord | RoomHistoryRecord;

export interface RoomFile {
  name: string;
  size: number;
  mime_type: string;
  download_url: string;
}

export interface RoomMessage {
  id: number;
  kind: "text" | "file";
  source: "user" | "clipboard";
  text: string;
  created_at: string;
  sender: RoomMember;
  file?: RoomFile;
}
