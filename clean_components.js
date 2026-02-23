const fs = require('fs');
const path = require('path');

function walk(dir) {
    let results = [];
    const list = fs.readdirSync(dir);
    list.forEach(function(file) {
        file = dir + '/' + file;
        const stat = fs.statSync(file);
        if (stat && stat.isDirectory()) { 
            results = results.concat(walk(file));
        } else { 
            if (file.endsWith('.json')) results.push(file);
        }
    });
    return results;
}

const files = walk('./weapp/miniprogram');
let modifiedCount = 0;

files.forEach(file => {
    try {
        let content = fs.readFileSync(file, 'utf8');
        if (content.trim().startsWith('{')) {
            const json = JSON.parse(content);
            let changed = false;
            if (json.usingComponents) {
                if (json.usingComponents['t-collapse']) { delete json.usingComponents['t-collapse']; changed = true; }
                if (json.usingComponents['t-collapse-panel']) { delete json.usingComponents['t-collapse-panel']; changed = true; }
                if (json.usingComponents['t-navbar']) { delete json.usingComponents['t-navbar']; changed = true; }
                if (json.usingComponents['category-tabs'] && !file.includes('category-tabs')) { delete json.usingComponents['category-tabs']; changed = true; }
            }
            if (changed) {
                fs.writeFileSync(file, JSON.stringify(json, null, 4), 'utf8');
                modifiedCount++;
                console.log('Cleaned: ' + file);
            }
        }
    } catch(e) { /* ignore parse errors for non-standard json */ }
});

console.log('Cleaned ' + modifiedCount + ' files completely.');
