/**
 * 列出 Swagger 中所有 definitions，按前缀分组
 */
const fs = require('fs')
const path = require('path')

const swagger = JSON.parse(fs.readFileSync(path.join(__dirname, '../docs/swagger.json'), 'utf-8'))

const definitions = Object.keys(swagger.definitions || {})

// 按前缀分组
const groups = {}
definitions.forEach(name => {
    const prefix = name.split('.')[0] || 'other'
    if (!groups[prefix]) groups[prefix] = []
    groups[prefix].push(name)
})

console.log(`总共 ${definitions.length} 个 definitions\n`)

// 输出分组统计
Object.entries(groups).sort((a, b) => b[1].length - a[1].length).forEach(([prefix, items]) => {
    console.log(`${prefix}: ${items.length} 个`)
})

console.log('\n--- api 前缀的详细列表 ---\n')

// 输出 api 前缀的所有定义，按字母排序
const apiDefs = groups['api'] || []
apiDefs.sort().forEach(name => {
    const shortName = name.replace('api.', '')
    console.log(shortName)
})

// 保存到文件
const output = {
    total: definitions.length,
    groups: Object.fromEntries(
        Object.entries(groups).map(([k, v]) => [k, v.sort()])
    )
}
fs.writeFileSync(
    path.join(__dirname, '../docs/swagger-definitions-list.json'),
    JSON.stringify(output, null, 2)
)
console.log('\n已保存到 docs/swagger-definitions-list.json')
