#!/bin/bash
# 测试WebSocket实时推送完整流程
# 使用方法: ./scripts/test_websocket_push.sh

set -e

echo "=== WebSocket实时推送测试 ==="
echo ""

# 配置
REDIS_HOST=${REDIS_HOST:-localhost:6379}
API_BASE_URL=${API_BASE_URL:-http://localhost:8080}

echo "📋 测试配置:"
echo "  Redis: $REDIS_HOST"
echo "  API: $API_BASE_URL"
echo ""

# 1. 模拟worker发布通知推送请求
echo "1️⃣ 模拟worker通过Redis Pub/Sub发布推送请求"
cat << 'EOF' | redis-cli -h ${REDIS_HOST%%:*} -p ${REDIS_HOST##*:}
PUBLISH notification:rider:1 '{"entity_type":"rider","entity_id":1,"message":{"type":"notification","data":{"id":123,"user_id":100,"type":"delivery","title":"测试通知","content":"这是一条测试通知","is_read":false,"created_at":"2025-11-27T15:00:00Z"},"timestamp":"2025-11-27T15:00:00Z"}}'
EOF
echo "   ✅ 推送请求已发布到 notification:rider:1"
echo ""

# 2. 验证Redis订阅
echo "2️⃣ 验证Redis订阅状态 (5秒后超时)"
timeout 5s redis-cli -h ${REDIS_HOST%%:*} -p ${REDIS_HOST##*:} PUBSUB CHANNELS "notification:*" || true
echo ""

# 3. 显示连接的WebSocket客户端数量（需要API支持）
echo "3️⃣ WebSocket Hub状态:"
echo "   提示: 在API服务器日志中查看连接状态"
echo "   预期日志: 'rider {rider_id} connected' 或 'merchant {merchant_id} connected'"
echo ""

echo "=== 手动测试步骤 ==="
echo ""
echo "📱 前端测试:"
echo "   1. 使用WebSocket客户端连接: ws://localhost:8080/v1/ws"
echo "   2. 在Authorization header中携带JWT token (角色必须是rider或merchant)"
echo "   3. 触发一个通知事件 (如: 商户接单)"
echo "   4. 检查是否实时收到WebSocket消息"
echo ""

echo "🧪 后端测试:"
echo "   1. 启动API服务器: make server"
echo "   2. 启动worker: make worker"
echo "   3. 创建订单并支付"
echo "   4. 检查日志输出:"
echo "      - worker: 'published notification push request to Redis'"
echo "      - API: 'pushed notification to rider/merchant via WebSocket'"
echo ""

echo "🔍 调试命令:"
echo "   # 监听Redis Pub/Sub"
echo "   redis-cli PSUBSCRIBE 'notification:*'"
echo ""
echo "   # 查看API服务器日志"
echo "   tail -f logs/api.log | grep -E '(WebSocket|notification)'"
echo ""
echo "   # 查看worker日志"
echo "   tail -f logs/worker.log | grep -E 'notification'"
echo ""
