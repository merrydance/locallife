const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')
const publicConfigPath = path.join(rootDir, 'project.config.json')
const privateConfigPath = path.join(rootDir, 'project.private.config.json')

function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'))
}

const publicConfig = readJson(publicConfigPath)
const publicLibVersion = publicConfig.libVersion

assert(
  publicLibVersion,
  'project.config.json must declare the Mini Program base library version'
)

if (fs.existsSync(privateConfigPath)) {
  const privateConfig = readJson(privateConfigPath)
  const privateLibVersion = privateConfig.libVersion

  assert.notStrictEqual(
    privateLibVersion,
    '3.16.0',
    'project.private.config.json must not pin DevTools to base library 3.16.0; this version is linked to WAServiceMainContext timeout noise'
  )

  assert(
    privateLibVersion === undefined || privateLibVersion === publicLibVersion,
    `project.private.config.json libVersion (${privateLibVersion}) must match project.config.json (${publicLibVersion}) or be omitted`
  )
}

console.log('check-project-lib-version tests passed')
