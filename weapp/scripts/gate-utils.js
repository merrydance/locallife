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

module.exports = {
  repoRoot,
  weappRoot,
  getDiffBase,
  getChangedEntries,
  pathExistsInRevision,
  readFileIfExists,
  listFiles
}