const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const takeoutHomeWxml = fs.readFileSync(path.join(repoRoot, 'miniprogram/pages/takeout/index.wxml'), 'utf8')
const takeoutSearchWxml = fs.readFileSync(path.join(repoRoot, 'miniprogram/pages/takeout/search/index.wxml'), 'utf8')

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

assert(
  /<view\s+class="search-section"[\s\S]*<t-search[\s\S]*placeholder="搜索菜品、商家"[\s\S]*disabled="\{\{true\}\}"[\s\S]*<view\s+class="search-hit-layer"[\s\S]*bindtap="onSearchTap"[\s\S]*aria-role="button"[\s\S]*aria-label="搜索菜品、商家"/.test(takeoutHomeWxml),
  'takeout home search shortcut should use a dedicated hit layer wired to onSearchTap'
)

assert(
  /<t-search[\s\S]*placeholder="搜索菜品、商家"[\s\S]*action="搜索"[\s\S]*bind:submit="onSearch"[\s\S]*bind:action-click="onSearch"/.test(takeoutSearchWxml),
  'takeout search page should expose both keyboard submit and visible search action'
)

console.log('takeout home search entry tests passed')
