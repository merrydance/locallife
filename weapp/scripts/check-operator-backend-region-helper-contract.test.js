const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..', '..')
const auditDocPath = path.join(
  repoRoot,
  'artifacts/operator-capability-audit/operator-backend-weapp-capability-audit-2026-06-20.md'
)
const apiDir = path.join(repoRoot, 'locallife/api')

const knownHelperOwners = new Set([
  'delivery_fee.go',
  'operator_rules.go',
  'operator_realtime.go',
  'operator_stats.go'
])

function read(file) {
  return fs.readFileSync(file, 'utf8')
}

function listGoFiles(dir) {
  return fs
    .readdirSync(dir)
    .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
    .map((file) => path.join(dir, file))
}

function collectGetOperatorRegionIDCallsites() {
  const callsites = []
  for (const filePath of listGoFiles(apiDir)) {
    const fileName = path.basename(filePath)
    const source = read(filePath)
    const lines = source.split(/\r?\n/)
    lines.forEach((line, index) => {
      if (!line.includes('getOperatorRegionID(')) {
        return
      }
      if (line.includes('func (server *Server) getOperatorRegionID(')) {
        return
      }
      let functionName = ''
      for (let i = index; i >= 0; i -= 1) {
        const match = lines[i].match(/^func \(server \*Server\) ([A-Za-z0-9_]+)\(/)
        if (match) {
          functionName = match[1]
          break
        }
      }
      callsites.push({
        fileName,
        functionName,
        line: index + 1,
        lineText: line.trim()
      })
    })
  }
  return callsites
}

const auditDoc = read(auditDocPath)
const callsites = collectGetOperatorRegionIDCallsites()

assert(callsites.length > 0, 'operator backend helper guard should find getOperatorRegionID callsites')

for (const callsite of callsites) {
  assert(
    knownHelperOwners.has(callsite.fileName),
    `unexpected getOperatorRegionID callsite in ${callsite.fileName}:${callsite.line}; classify it in this gate and the audit matrix`
  )
  assert(
    callsite.functionName,
    `cannot identify enclosing handler/helper for ${callsite.fileName}:${callsite.line}`
  )
  assert(
    auditDoc.includes(`| ${callsite.fileName} | ${callsite.functionName} |`) &&
      auditDoc.includes(callsite.lineText) &&
      /必须单区|兼容兜底|待迁移聚合/.test(auditDoc),
    `missing operator region helper matrix entry for ${callsite.fileName}:${callsite.line} ${callsite.functionName} (${callsite.lineText})`
  )
}

assert(
  auditDoc.includes('#### OP-RISK-002-B 完成复核'),
  'audit doc must include OP-RISK-002-B review record after adding the helper guard'
)

console.log('check-operator-backend-region-helper-contract: backend operator region helper callsites are classified')
