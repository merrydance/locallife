const fs = require('fs')
const path = require('path')
const { repoRoot, getGateScope, getScopedFiles } = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const TDESIGN_TAG_REGEX = /<\s*(t-[a-z][a-z0-9-]*)\b/g

function findNearestConfigPath(relativePath) {
  const directConfig = relativePath.replace(/\.wxml$/, '.json')
  if (fs.existsSync(path.join(repoRoot, directConfig))) {
    return directConfig
  }

  let currentDir = path.dirname(relativePath)

  while (currentDir && currentDir !== '.' && SURFACE_ROOTS.some((root) => currentDir.startsWith(root))) {
    const absoluteDir = path.join(repoRoot, currentDir)
    if (!fs.existsSync(absoluteDir)) {
      break
    }

    const jsonFiles = fs.readdirSync(absoluteDir)
      .filter((name) => name.endsWith('.json'))
      .filter((name) => name !== 'component-policy.json')

    if (jsonFiles.length > 0) {
      const indexJson = jsonFiles.find((name) => name === 'index.json')
      const dirNamedJson = jsonFiles.find((name) => name === `${path.basename(currentDir)}.json`)
      const chosenFile = indexJson || dirNamedJson || (jsonFiles.length === 1 ? jsonFiles[0] : null)

      if (chosenFile) {
        return path.posix.join(currentDir, chosenFile)
      }
    }

    const parentDir = path.dirname(currentDir)
    if (parentDir === currentDir) {
      break
    }
    currentDir = parentDir
  }

  return null
}

function collectTDesignTags(content) {
  const tags = new Set()
  let match = TDESIGN_TAG_REGEX.exec(content)

  while (match) {
    tags.add(match[1])
    match = TDESIGN_TAG_REGEX.exec(content)
  }

  return Array.from(tags).sort()
}

function main() {
  const wxmlFiles = getScopedFiles({ roots: SURFACE_ROOTS, extensions: ['.wxml'] })

  if (wxmlFiles.length === 0) {
    console.log(`check-tdesign-component-declarations: no ${getGateScope() === 'changed' ? 'changed' : 'scannable'} Mini Program WXML files detected`)
    return
  }

  const failures = []

  for (const relativePath of wxmlFiles) {
    const absolutePath = path.join(repoRoot, relativePath)
    const content = fs.readFileSync(absolutePath, 'utf8')
    const tags = collectTDesignTags(content)

    if (tags.length === 0) {
      continue
    }

    const jsonRelativePath = findNearestConfigPath(relativePath)

    if (!jsonRelativePath) {
      failures.push({
        relativePath,
        reason: 'missing reachable JSON config for TDesign component declarations'
      })
      continue
    }

    const jsonAbsolutePath = path.join(repoRoot, jsonRelativePath)

    let config
    try {
      config = JSON.parse(fs.readFileSync(jsonAbsolutePath, 'utf8'))
    } catch (error) {
      failures.push({
        relativePath,
        reason: `${path.basename(jsonRelativePath)} is not valid JSON`
      })
      continue
    }

    const usingComponents = config.usingComponents || {}

    for (const tag of tags) {
      if (!Object.prototype.hasOwnProperty.call(usingComponents, tag)) {
        failures.push({
          relativePath,
          reason: `${tag} is used in WXML but missing from usingComponents in ${path.basename(jsonRelativePath)}`
        })
        continue
      }

      const componentPath = usingComponents[tag]
      if (typeof componentPath !== 'string' || !componentPath.startsWith('tdesign-miniprogram/')) {
        failures.push({
          relativePath,
          reason: `${tag} must map to an official tdesign-miniprogram component path in ${path.basename(jsonRelativePath)}`
        })
      }
    }
  }

  if (failures.length > 0) {
    console.error('TDesign declaration gate failed. Every t-* component used in WXML must be declared in the sibling JSON usingComponents block with an official tdesign-miniprogram path.')
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      console.error(`  - ${failure.reason}`)
    }

    process.exit(1)
  }

  console.log(`check-tdesign-component-declarations: validated ${wxmlFiles.length} WXML file(s)`) 
}

main()