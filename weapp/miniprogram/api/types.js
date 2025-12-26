"use strict";
/**
 * API通用类型定义
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.Status = exports.ErrorCode = void 0;
/**
 * 错误码枚举
 */
var ErrorCode;
(function (ErrorCode) {
    ErrorCode[ErrorCode["SUCCESS"] = 0] = "SUCCESS";
    ErrorCode[ErrorCode["BAD_REQUEST"] = 400] = "BAD_REQUEST";
    ErrorCode[ErrorCode["UNAUTHORIZED"] = 401] = "UNAUTHORIZED";
    ErrorCode[ErrorCode["FORBIDDEN"] = 403] = "FORBIDDEN";
    ErrorCode[ErrorCode["NOT_FOUND"] = 404] = "NOT_FOUND";
    ErrorCode[ErrorCode["INTERNAL_ERROR"] = 500] = "INTERNAL_ERROR";
    ErrorCode[ErrorCode["TOKEN_EXPIRED"] = 1001] = "TOKEN_EXPIRED";
})(ErrorCode || (exports.ErrorCode = ErrorCode = {}));
/**
 * 状态枚举
 */
var Status;
(function (Status) {
    Status["ACTIVE"] = "ACTIVE";
    Status["INACTIVE"] = "INACTIVE";
    Status["DELETED"] = "DELETED";
})(Status || (exports.Status = Status = {}));
