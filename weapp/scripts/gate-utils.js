const childProcess = require('child_process')
const fs = require('fs')
const path = require('path')

function runGit(command, cwd) {
  return childProcess.execSync(command, {
    cwd,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
}

const repoRoot = runGit('git rev-parse --show-toplevel', __dirname)
const weappRoot = path.join(repoRoot, 'weapp')

function normalizeRelativePath(filePath) {
  return filePath.replace(/\\/g, '/')
}

function getDiffBase() {
  if (process.env.GITHUB_BASE_REF) {
    const remoteBase = `origin/${process.env.GITHUB_BASE_REF}`

    try {
      runGit(`git rev-parse --verify ${remoteBase}`, repoRoot)
      return runGit(`git merge-base HEAD ${remoteBase}`, repoRoot)
    } catch (error) {
      return 'HEAD'
    }
  }

  if (process.env.GITHUB_EVENT_BEFORE && !/^0+$/.test(process.env.GITHUB_EVENT_BEFORE)) {
    return process.env.GITHUB_EVENT_BEFORE
  }

  return 'HEAD'
}

function splitLines(output) {
  return output ? output.split('\n').map((line) => line.trim()).filter(Boolean) : []
}

function getChangedEntries() {
  const diffBase = getDiffBase()

  if (diffBase !== 'HEAD') {
    const lines = splitLines(runGit(`git diff --name-status --diff-filter=ACMR ${diffBase}...HEAD`, repoRoot))
    return lines.map(parseNameStatusLine).filter(Boolean)
  }

  const tracked = splitLines(runGit('git diff --name-status --diff-filter=ACMR HEAD', repoRoot))
    .map(parseNameStatusLine)
    .filter(Boolean)

  const untracked = splitLines(runGit('git ls-files --others --exclude-standard', repoRoot)).map((filePath) => ({
    status: 'A',
    filePath
  }))

  return [...tracked, ...untracked]
}

function getGateScope() {
  return process.env.WEAPP_GATE_SCOPE === 'changed' ? 'changed' : 'all'
}

function parseNameStatusLine(line) {
  const parts = line.split(/\t+/)

  if (parts.length < 2) {
    return null
  }

  return {
    status: parts[0][0],
    filePath: parts[parts.length - 1]
  }
}

function pathExistsInRevision(revision, relativePath) {
  try {
    runGit(`git cat-file -e ${revision}:${relativePath}`, repoRoot)
    return true
  } catch (error) {
    return false
  }
}

function readFileAtRevision(revision, relativePath) {
  try {
    return runGit(`git show ${revision}:${relativePath}`, repoRoot)
  } catch (error) {
    return ''
  }
}

function readFileIfExists(filePath) {
  return fs.existsSync(filePath) ? fs.readFileSync(filePath, 'utf8') : ''
}

function listFiles(dirPath, extensions) {
  if (!fs.existsSync(dirPath)) {
    return []
  }

  return fs.readdirSync(dirPath)
    .filter((name) => extensions.includes(path.extname(name)))
    .map((name) => path.join(dirPath, name))
}

function listFilesRecursive(dirPath, extensions) {
  if (!fs.existsSync(dirPath)) {
    return []
  }

  const results = []
  const entries = fs.readdirSync(dirPath, { withFileTypes: true })

  for (const entry of entries) {
    const entryPath = path.join(dirPath, entry.name)

    if (entry.isDirectory()) {
      results.push(...listFilesRecursive(entryPath, extensions))
      continue
    }

    if (extensions.includes(path.extname(entry.name))) {
      results.push(entryPath)
    }
  }

  return results
}

function getScopedFiles({ roots, extensions }) {
  if (getGateScope() === 'changed') {
    return Array.from(new Set(
      getChangedEntries()
        .map((entry) => normalizeRelativePath(entry.filePath))
        .filter((filePath) => roots.some((root) => filePath.startsWith(root)))
        .filter((filePath) => extensions.includes(path.extname(filePath)))
    )).sort()
  }

  return Array.from(new Set(
    roots.flatMap((root) => listFilesRecursive(path.join(repoRoot, root), extensions)
      .map((filePath) => normalizeRelativePath(path.relative(repoRoot, filePath))))
  )).sort()
}

module.exports = {
  repoRoot,
  weappRoot,
  getDiffBase,
  getChangedEntries,
  getGateScope,
  getScopedFiles,
  pathExistsInRevision,
  readFileAtRevision,
  readFileIfExists,
  listFiles,
  listFilesRecursive,
  normalizeRelativePath
}