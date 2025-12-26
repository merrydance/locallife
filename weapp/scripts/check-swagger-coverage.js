/**
 * 检查 Swagger 定义在 TS 中的覆盖情况
 */

const fs = require('fs');
const path = require('path');

// 读取 Swagger 定义列表
const swaggerDefsPath = path.join(__dirname, '../docs/swagger-definitions-list.json');
const swaggerDefs = JSON.parse(fs.readFileSync(swaggerDefsPath, 'utf-8'));

// 只关注 api 前缀的定义
const apiDefsRaw = swaggerDefs.groups?.api || [];
// 去掉 "api." 前缀
const apiDefs = apiDefsRaw.map(d => d.replace('api.', ''));

console.log(`Swagger 中共有 ${apiDefs.length} 个 api.xxx 定义\n`);

// 读取验证报告获取已匹配的类型
const reportPath = path.join(__dirname, '../docs/api-swagger-diff-report.md');
const reportContent = fs.readFileSync(reportPath, 'utf-8');

// 解析已匹配的 Swagger 定义
const matchedSwaggerDefs = new Set();
const matchRegex = /→ api\.(\w+)/g;
let match;
while ((match = matchRegex.exec(reportContent)) !== null) {
    matchedSwaggerDefs.add(match[1]);
}

console.log(`TS 中已匹配 ${matchedSwaggerDefs.size} 个 Swagger 定义\n`);

// 找出未被覆盖的 Swagger 定义
const uncoveredDefs = apiDefs.filter(def => !matchedSwaggerDefs.has(def));

console.log('='.repeat(60));
console.log(`❌ 未被 TS 覆盖的 Swagger 定义: ${uncoveredDefs.length} 个`);
console.log('='.repeat(60));

// 按类别分组
const categories = {
    request: [],
    response: [],
    other: []
};

for (const def of uncoveredDefs) {
    const lowerDef = def.toLowerCase();
    if (lowerDef.includes('request') || lowerDef.includes('body')) {
        categories.request.push(def);
    } else if (lowerDef.includes('response') || lowerDef.includes('row') || lowerDef.includes('item')) {
        categories.response.push(def);
    } else {
        categories.other.push(def);
    }
}

console.log(`\n请求类型 (${categories.request.length} 个):`);
categories.request.forEach(d => console.log(`  - api.${d}`));

console.log(`\n响应类型 (${categories.response.length} 个):`);
categories.response.forEach(d => console.log(`  - api.${d}`));

console.log(`\n其他类型 (${categories.other.length} 个):`);
categories.other.forEach(d => console.log(`  - api.${d}`));

// 保存报告
const report = {
    summary: {
        totalSwaggerDefs: apiDefs.length,
        matchedInTs: matchedSwaggerDefs.size,
        uncovered: uncoveredDefs.length
    },
    uncoveredDefs: {
        request: categories.request,
        response: categories.response,
        other: categories.other
    }
};

fs.writeFileSync(
    path.join(__dirname, '../docs/swagger-coverage-report.json'),
    JSON.stringify(report, null, 2)
);

console.log('\n报告已保存到: docs/swagger-coverage-report.json');
