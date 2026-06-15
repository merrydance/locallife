const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')

const requiredSections = [
  '## Device And Build',
  '## Fixture Data',
  '## Execution Evidence',
  '## Result',
]

const weappRoot = path.join(__dirname, '..')

const requiredNeedles = [
  'Device model:',
  'WeChat version:',
  'Mini Program build:',
  'Backend environment:',
  'Operator:',
  'Session ID:',
  'Order ID:',
  'Payment Order ID:',
  '1. QR scan/open session:',
  '2. Dine-in checkout saves pending context:',
  '3. Payment result reload while pending_confirmation:',
  '4. Backend payment reaches paid:',
  '5. Paid polling triggers session checkout:',
  '6. Backend session readback is closed/non-actionable:',
  'Screenshot or recording evidence:',
  'Backend verification:',
  'Verdict:',
]

const forbiddenPlaceholders = [
  '<fill',
  '<todo',
  '<required',
  'TBD',
  'TODO',
]

function readEvidence(filePath) {
  assert(filePath, 'usage: node scripts/check-dine-in-device-e2e-evidence.test.js <evidence.md>')
  return fs.readFileSync(filePath, 'utf8')
}

function assertPackageWiring() {
  const pkg = JSON.parse(fs.readFileSync(path.join(weappRoot, 'package.json'), 'utf8'))
  assert(
    pkg.scripts['check:dine-in-device-e2e-evidence'],
    'package.json must expose the dine-in device E2E evidence check'
  )
  assert(
    pkg.scripts['quality:check'].includes('check:dine-in-device-e2e-evidence'),
    'quality:check must run the dine-in device E2E evidence check'
  )
}

function assertEvidence(content) {
  for (const section of requiredSections) {
    assert(content.includes(section), `missing required section: ${section}`)
  }
  for (const needle of requiredNeedles) {
    assert(content.includes(needle), `missing required evidence field: ${needle}`)
  }
  for (const placeholder of forbiddenPlaceholders) {
    assert(!content.toLowerCase().includes(placeholder.toLowerCase()), `evidence still contains placeholder: ${placeholder}`)
  }
  assert(/Verdict:\s*(pass|fail)/i.test(content), 'Verdict must be pass or fail')
}

function runFixtureSelfCheck() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'dine-in-device-e2e-evidence-'))
  const evidencePath = path.join(tmpDir, 'evidence.md')
  const evidence = `# Dine-In Checkout Device E2E Evidence

## Device And Build

Device model: iPhone 15
WeChat version: 8.0.50
Mini Program build: trial 2026-06-15.1
Backend environment: staging
Operator: release-owner

## Fixture Data

Session ID: 1001
Order ID: 2002
Payment Order ID: 3003

## Execution Evidence

1. QR scan/open session: pass
2. Dine-in checkout saves pending context: pass
3. Payment result reload while pending_confirmation: pass
4. Backend payment reaches paid: pass
5. Paid polling triggers session checkout: pass
6. Backend session readback is closed/non-actionable: pass
Screenshot or recording evidence: artifacts/private/dine-in-device-e2e.mp4
Backend verification: GET /v1/dining-sessions/1001 returned closed

## Result

Verdict: pass
`
  fs.writeFileSync(evidencePath, evidence)
  assertEvidence(readEvidence(evidencePath))
  fs.rmSync(tmpDir, { recursive: true, force: true })
}

function main() {
  assertPackageWiring()

  const evidencePath = process.argv[2]
  if (!evidencePath) {
    runFixtureSelfCheck()
    console.log('check-dine-in-device-e2e-evidence: evidence schema contract passed')
    return
  }
  const content = readEvidence(evidencePath)
  assertEvidence(content)
  const kind = content.includes('Status: template only') ? 'template schema' : 'evidence'
  console.log(`check-dine-in-device-e2e-evidence: ${evidencePath} ${kind} is complete`)
}

main()
