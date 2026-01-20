"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getBicyclingDirection = getBicyclingDirection;
const request_1 = require("../utils/request");
function getBicyclingDirection(params) {
    return (0, request_1.request)({
        url: '/v1/location/direction/bicycling',
        method: 'GET',
        data: params
    });
}
