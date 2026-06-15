const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')

const requiredSections = [
  '## Target Flow',
  '## Device And Build',
  '## Fixture Data',
  '## Provider Evidence',
  '## Recovery And Visibility',
  '## Result',
]

const requiredNeedles = [
  'Flow type:',
  'Device model:',
  'WeChat version:',
  'Mini Program build:',
  'Backend environment:',
  'Operator:',
  'Order or reservation ID:',
  'Payment Order ID:',
  'Provider fact ID:',
  'Provider application ID:',
  'Callback or query evidence:',
  'Baofu evidence gate command:',
  '1. Client leaves or reloads payment result:',
  '2. Backend payment truth reaches terminal state:',
  '3. Detail page readback:',
  '4. List page readback:',
  'Screenshot or recording evidence:',
  'Backend verification:',
  'Verdict:',
]

const allowedFlowTypes = ['takeout', 'reservation', 'reservation_addon']
const forbiddenPlaceholders = [
  '<fill',
  '<todo',
  '<required',
  'TBD',
  'TODO',
  'no provider run recorded',
  'no device run recorded',
  'Keep this template',
]

const weappRoot = path.join(__dirname, '..')

function readEvidence(filePath) {
  assert(filePath, 'usage: node scripts/check-customer-checkout-provider-e2e-evidence.test.js <evidence.md>')
  return fs.readFileSync(filePath, 'utf8')
}

function assertPackageWiring() {
  const pkg = JSON.parse(fs.readFileSync(path.join(weappRoot, 'package.json'), 'utf8'))
  assert(
    pkg.scripts['check:customer-checkout-provider-e2e-evidence'],
    'package.json must expose the customer checkout provider E2E evidence check'
  )
  assert(
    pkg.scripts['quality:check'].includes('check:customer-checkout-provider-e2e-evidence'),
    'quality:check must run the customer checkout provider E2E evidence check'
  )
}

function assertEvidence(content) {
  for (const section of requiredSections) {
    assert(content.includes(section), `missing required section: ${section}`)
  }
  for (const needle of requiredNeedles) {
    assert(content.includes(needle), `missing required evidence field: ${needle}`)
  }
  assert(
    allowedFlowTypes.some((flowType) => new RegExp(`Flow type:\\s*${flowType}\\b`, 'i').test(content)),
    `Flow type must be one of: ${allowedFlowTypes.join(', ')}`
  )
  assert(content.includes('scripts/baofu_provider_evidence_gate.sh'), 'Baofu evidence gate command must be recorded')
  assert(!/Status:\s*template only/i.test(content), 'template evidence cannot be used as release evidence')
  for (const placeholder of forbiddenPlaceholders) {
    assert(!content.toLowerCase().includes(placeholder.toLowerCase()), `evidence still contains placeholder: ${placeholder}`)
  }
  assert(!/:\s*record\b/im.test(content), 'evidence still contains template instructions')
  assert(/Verdict:\s*pass\b/i.test(content), 'Verdict must be pass for release evidence')
}

function assertEvidenceRejected(content, expectedMessage) {
  assert.throws(() => assertEvidence(content), expectedMessage)
}

function runFixtureSelfCheck() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'customer-checkout-provider-e2e-evidence-'))
  const evidencePath = path.join(tmpDir, 'evidence.md')
  const evidence = `# Customer Checkout Provider E2E Evidence

## Target Flow

Flow type: takeout

## Device And Build

Device model: iPhone 15
WeChat version: 8.0.50
Mini Program build: trial 2026-06-15.1
Backend environment: staging
Operator: release-owner

## Fixture Data

Order or reservation ID: 2002
Payment Order ID: 3003
Provider fact ID: 4004
Provider application ID: 5005

## Provider Evidence

Callback or query evidence: Baofu callback row applied with masked provider ids
Baofu evidence gate command: PATH="/usr/local/go/bin:$PATH" scripts/baofu_provider_evidence_gate.sh --capability payment --fact-id 4004 --application-id 5005 --payment-order-id 3003 --ledger-row --evidence-kind callback --ledger-date 2026-06-15 --ledger-env production --ledger-endpoint https://llapi.merrydance.cn/v1/webhooks/baofu/payment --ledger-ack OK --ledger-commit 994c10db --ledger-notes controlled-customer-checkout-e2e

## Recovery And Visibility

1. Client leaves or reloads payment result: pass
2. Backend payment truth reaches terminal state: pass
3. Detail page readback: pass
4. List page readback: pass
Screenshot or recording evidence: artifacts/private/customer-checkout-provider-e2e.mp4
Backend verification: GET /v1/orders/2002 returned paid

## Result

Verdict: pass
`
  fs.writeFileSync(evidencePath, evidence)
  assertEvidence(readEvidence(evidencePath))
  assertEvidenceRejected(
    evidence.replace('Verdict: pass', 'Status: template only; no provider run recorded\nVerdict: pass'),
    /template evidence/i
  )
  assertEvidenceRejected(evidence.replace('Verdict: pass', 'Verdict: fail'), /Verdict must be pass/i)
  assertEvidenceRejected(
    evidence.replace('Flow type: takeout', 'Flow type: unsupported'),
    /Flow type must be one of/i
  )
  assertEvidenceRejected(
    evidence.replace('Device model: iPhone 15', 'Device model: record the physical device model'),
    /template instructions/i
  )
  fs.rmSync(tmpDir, { recursive: true, force: true })
}

function main() {
  assertPackageWiring()

  const evidencePath = process.argv[2]
  if (!evidencePath) {
    runFixtureSelfCheck()
    console.log('check-customer-checkout-provider-e2e-evidence: release evidence contract passed')
    return
  }
  const content = readEvidence(evidencePath)
  assertEvidence(content)
  console.log(`check-customer-checkout-provider-e2e-evidence: ${evidencePath} release evidence is complete`)
}

main()
