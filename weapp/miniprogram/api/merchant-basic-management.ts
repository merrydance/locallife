import { UploadService } from './upload'

export async function uploadImage(filePath: string, type: string = 'common'): Promise<string> {
    return UploadService.uploadImage(filePath, type)
}
