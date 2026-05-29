const assert = require('assert')
const fs = require('fs')
const path = require('path')

const repoRoot = path.join(__dirname, '..')

const websocketFiles = [
  ['merchant', 'miniprogram/pages/merchant/_main_shared/utils/websocket.ts'],
  ['platform', 'miniprogram/pages/platform/_main_shared/utils/websocket.ts'],
  ['rider', 'miniprogram/pages/rider/_main_shared/utils/websocket.ts']
]

for (const [role, relativePath] of websocketFiles) {
  const sourcePath = path.join(repoRoot, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const connectIndex = source.indexOf('async connect(')
  const ensureIndex = source.indexOf('await ensureValidToken()', connectIndex)
  const tokenIndex = source.indexOf('const token = getToken()', connectIndex)
  const connectingIndex = source.indexOf('this.isConnecting = true', connectIndex)

  assert(
    source.includes('private lastUrlOrPath?: string'),
    `${role} WebSocket manager must remember the last explicit endpoint for reconnect`
  )
  assert(
    source.includes("import { ensureValidToken } from '../../../../utils/request-auth-refresh'"),
    `${role} WebSocket manager must import ensureValidToken`
  )
  assert(connectIndex >= 0, `${role} WebSocket connect must be async`)
  assert(ensureIndex >= 0, `${role} WebSocket connect must await ensureValidToken`)
  assert(tokenIndex >= 0, `${role} WebSocket connect must read the token after refresh`)
  assert(
    connectingIndex >= 0 && connectingIndex < ensureIndex,
    `${role} WebSocket connect must mark isConnecting before async token refresh`
  )
  assert(
    ensureIndex < tokenIndex,
    `${role} WebSocket connect must refresh before reading the token`
  )
  assert(
    source.includes('if (urlOrPath !== undefined)') &&
      source.includes('this.lastUrlOrPath = urlOrPath') &&
      source.includes('const targetUrlOrPath = this.lastUrlOrPath') &&
      source.includes('this.resolveWebSocketUrl(token, targetUrlOrPath, this.lastSequence)'),
    `${role} WebSocket manager must preserve custom endpoints across reconnect`
  )
  assert(
    source.includes("logger.warn('WebSocket: Token refresh failed before connection'"),
    `${role} WebSocket manager must log token refresh failures before opening a socket`
  )
  assert(
    source.includes("this.notifyConnectionBlocked('token refresh failed')"),
    `${role} WebSocket manager must surface auth refresh failures without reconnecting with an old token`
  )
  assert(
    source.includes('void this.connect(this.lastUrlOrPath)'),
    `${role} WebSocket reconnect timer must handle async connect without a floating rejection`
  )
}

console.log('check-websocket-auth-refresh tests passed')
