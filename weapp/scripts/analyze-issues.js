const issues = require('../docs/api-swagger-issues.json')

// 按文件统计
const byFile = {}
issues.forEach(i => { byFile[i.file] = (byFile[i.file] || 0) + 1 })

console.log('按文件统计问题数 (Top 15):')
Object.entries(byFile)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 15)
    .forEach(([f, c]) => console.log(`  ${f}: ${c}`))

// 按类型统计
const byType = { missing: 0, extra: 0 }
issues.forEach(i => { byType[i.type]++ })

console.log('\n按类型统计:')
console.log(`  缺少字段 (missing): ${byType.missing}`)
console.log(`  多余字段 (extra): ${byType.extra}`)

// 高频缺失字段
const missingFields = {}
issues.filter(i => i.type === 'missing').forEach(i => {
    missingFields[i.field] = (missingFields[i.field] || 0) + 1
})

console.log('\n高频缺失字段 (Top 10):')
Object.entries(missingFields)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 10)
    .forEach(([f, c]) => console.log(`  ${f}: ${c}次`))

// 高频多余字段
const extraFields = {}
issues.filter(i => i.type === 'extra').forEach(i => {
    extraFields[i.field] = (extraFields[i.field] || 0) + 1
})

console.log('\n高频多余字段 (Top 10):')
Object.entries(extraFields)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 10)
    .forEach(([f, c]) => console.log(`  ${f}: ${c}次`))
