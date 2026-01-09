/**
 * 检查未匹配 Swagger 的类型是否在代码中被使用
 */

const fs = require('fs');
const path = require('path');

// 读取 issues 文件获取未找到的类型
const issuesPath = path.join(__dirname, '../docs/api-swagger-diff-report.md');
const reportContent = fs.readFileSync(issuesPath, 'utf-8');

// 解析未找到的类型
const notFoundTypes = [];
const lines = reportContent.split('\n');
let currentFile = '';

for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // 匹配文件名
    const fileMatch = line.match(/^## (.+\.ts)$/);
    if (fileMatch) {
        currentFile = fileMatch[1];
        continue;
    }

    // 匹配未找到的类型
    const typeMatch = line.match(/^### ❓ (.+)$/);
    if (typeMatch) {
        notFoundTypes.push({
            file: currentFile,
            typeName: typeMatch[1]
        });
    }
}

console.log(`找到 ${notFoundTypes.length} 个未匹配 Swagger 的类型\n`);

// 搜索目录
const searchDirs = [
    path.join(__dirname, '../miniprogram/pages'),
    path.join(__dirname, '../miniprogram/components'),
    path.join(__dirname, '../miniprogram/services'),
    path.join(__dirname, '../miniprogram/adapters'),
    path.join(__dirname, '../miniprogram/models'),
    path.join(__dirname, '../miniprogram/utils'),
];

// 获取所有 TS 文件
function getAllTsFiles(dir) {
    const files = [];
    if (!fs.existsSync(dir)) return files;

    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
            files.push(...getAllTsFiles(fullPath));
        } else if (item.name.endsWith('.ts') && !item.name.endsWith('.d.ts')) {
            files.push(fullPath);
        }
    }
    return files;
}

// 收集所有非 API 目录的 TS 文件
const allTsFiles = [];
for (const dir of searchDirs) {
    allTsFiles.push(...getAllTsFiles(dir));
}

// 也检查 API 目录内的交叉引用
const apiDir = path.join(__dirname, '../miniprogram/api');
const apiFiles = getAllTsFiles(apiDir);

console.log(`搜索 ${allTsFiles.length} 个业务文件和 ${apiFiles.length} 个 API 文件\n`);

// 检查每个类型的使用情况
const results = {
    used: [],
    unused: [],
    onlyInApi: []  // 只在 API 文件中使用（可能是内部类型）
};

for (const { file, typeName } of notFoundTypes) {
    const apiFilePath = path.join(__dirname, '../miniprogram/api', file);

    // 在业务文件中搜索
    let usedInBusiness = false;
    let usedInApi = false;
    const usageLocations = [];

    // 搜索模式：类型名作为类型注解或导入
    const patterns = [
        new RegExp(`\\b${typeName}\\b`, 'g'),  // 类型名
    ];

    // 检查业务文件
    for (const tsFile of allTsFiles) {
        const content = fs.readFileSync(tsFile, 'utf-8');
        for (const pattern of patterns) {
            if (pattern.test(content)) {
                usedInBusiness = true;
                usageLocations.push(path.relative(path.join(__dirname, '..'), tsFile));
                break;
            }
        }
    }

    // 检查其他 API 文件（排除定义文件本身）
    for (const apiFile of apiFiles) {
        if (path.basename(apiFile) === file) continue;  // 跳过定义文件

        const content = fs.readFileSync(apiFile, 'utf-8');
        for (const pattern of patterns) {
            if (pattern.test(content)) {
                usedInApi = true;
                usageLocations.push(path.relative(path.join(__dirname, '..'), apiFile));
                break;
            }
        }
    }

    const typeInfo = {
        file,
        typeName,
        usageLocations: usageLocations.slice(0, 5)  // 最多显示5个位置
    };

    if (usedInBusiness) {
        results.used.push(typeInfo);
    } else if (usedInApi) {
        results.onlyInApi.push(typeInfo);
    } else {
        results.unused.push(typeInfo);
    }
}

// 输出结果
console.log('='.repeat(60));
console.log(`✅ 在业务代码中使用的类型: ${results.used.length} 个`);
console.log('='.repeat(60));
for (const item of results.used) {
    console.log(`  ${item.file} -> ${item.typeName}`);
    if (item.usageLocations.length > 0) {
        console.log(`    使用位置: ${item.usageLocations.join(', ')}`);
    }
}

console.log('\n' + '='.repeat(60));
console.log(`⚠️ 只在 API 文件间引用的类型: ${results.onlyInApi.length} 个`);
console.log('='.repeat(60));
for (const item of results.onlyInApi) {
    console.log(`  ${item.file} -> ${item.typeName}`);
}

console.log('\n' + '='.repeat(60));
console.log(`❌ 未使用的类型: ${results.unused.length} 个`);
console.log('='.repeat(60));
for (const item of results.unused) {
    console.log(`  ${item.file} -> ${item.typeName}`);
}

// 保存详细报告
const report = {
    summary: {
        total: notFoundTypes.length,
        usedInBusiness: results.used.length,
        onlyInApi: results.onlyInApi.length,
        unused: results.unused.length
    },
    used: results.used,
    onlyInApi: results.onlyInApi,
    unused: results.unused
};

fs.writeFileSync(
    path.join(__dirname, '../docs/unused-types-report.json'),
    JSON.stringify(report, null, 2)
);

console.log('\n详细报告已保存到: docs/unused-types-report.json');
