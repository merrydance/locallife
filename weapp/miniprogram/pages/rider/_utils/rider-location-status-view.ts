import type { RiderLiveLocationState } from './rider-live-location'
import { buildStatusTagView, resolveStatusTagTheme, type StatusTagTheme } from '../_main_shared/utils/status-tag'

type RiderLocationUploadState = RiderLiveLocationState['uploadState']

export interface RiderLocationStatusView {
  text: string
  theme: StatusTagTheme
  needsPermission: boolean
}

export function getRiderLocationStatusView(uploadState: RiderLocationUploadState): RiderLocationStatusView {
  switch (uploadState) {
    case 'tracking':
      return {
        text: '定位正常',
        theme: buildStatusTagView('定位正常', 'success').theme,
        needsPermission: false
      }
    case 'uploading':
      return {
        text: '正在上传位置',
        theme: buildStatusTagView('正在上传位置', 'info').theme,
        needsPermission: false
      }
    case 'retrying':
      return {
        text: '网络恢复后会自动补发',
        theme: buildStatusTagView('网络恢复后会自动补发', 'warning').theme,
        needsPermission: false
      }
    case 'permission_required':
      return {
        text: '需要开启定位权限',
        theme: buildStatusTagView('需要开启定位权限', 'danger').theme,
        needsPermission: true
      }
    case 'starting':
      return {
        text: '正在开启连续定位',
        theme: buildStatusTagView('正在开启连续定位', 'warning').theme,
        needsPermission: false
      }
    default:
      return {
        text: '等待连续定位启动',
        theme: resolveStatusTagTheme('neutral'),
        needsPermission: false
      }
  }
}