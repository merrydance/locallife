
import { uploadMedia, MediaUploadResult } from '../utils/media'

/**
 * 通用文件上传服务（媒体服务三步流程）
 */
export class UploadService {

    /**
     * 上传图片
     * @param filePath 本地文件路径
     * @param type 图片用途，即 mediaCategory，如 'avatar' | 'review' 等
     * @returns { mediaId, displayUrl, urls }
     */
    static async uploadImage(filePath: string, type: string = 'avatar'): Promise<MediaUploadResult> {
        return uploadMedia(filePath, {
            businessType: 'user',
            mediaCategory: type
        })
    }
}
