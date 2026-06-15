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
  'no device run recorded',
  'Keep this template',
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
  assertEvidenceRejected(
    evidence.replace('Verdict: pass', 'Status: template only; no device run recorded\nVerdict: pass'),
    /template evidence/i
  )
  assertEvidenceRejected(evidence.replace('Verdict: pass', 'Verdict: fail'), /Verdict must be pass/i)
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
    console.log('check-dine-in-device-e2e-evidence: release evidence contract passed')
    return
  }
  const content = readEvidence(evidencePath)
  assertEvidence(content)
  console.log(`check-dine-in-device-e2e-evidence: ${evidencePath} release evidence is complete`)
}

main()
