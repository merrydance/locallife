import {
  confirmWebLoginSession,
  getUserInfo,
  getWebLoginSessionStatus,
  updateUserInfo,
  type UserWorkbenchResponse
} from '../api/auth'
import { notificationService } from '../api/notification'
import { bindMerchant } from '../api/personal'
import { UploadService } from '../api/upload'

export type UserWorkbenchProfile = UserWorkbenchResponse

export function fetchUserProfile() {
  return getUserInfo()
}

export function updateUserProfile(payload: Parameters<typeof updateUserInfo>[0]) {
  return updateUserInfo(payload)
}

export function fetchWebLoginSessionStatus(code: string) {
  return getWebLoginSessionStatus(code)
}

export function confirmWebLoginSessionCode(code: string, sig: string, ts: number) {
  return confirmWebLoginSession(code, sig, ts)
}

export function bindMerchantInviteCode(code: string) {
  return bindMerchant(code)
}

export function fetchUnreadNotificationCount() {
  return notificationService.getUnreadCount()
}

export function uploadAvatarImage(filePath: string) {
  return UploadService.uploadImage(filePath, 'avatar')
}