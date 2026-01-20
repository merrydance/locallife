"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.uploadImage = uploadImage;
const upload_1 = require("./upload");
async function uploadImage(filePath, type = 'common') {
    return upload_1.UploadService.uploadImage(filePath, type);
}
