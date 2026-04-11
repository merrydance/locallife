#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

function parseArgs(argv) {
  const args = {};
  for (let index = 2; index < argv.length; index += 1) {
    const current = argv[index];
    if (!current.startsWith('--')) {
      continue;
    }

    const key = current.slice(2);
    const next = argv[index + 1];
    if (!next || next.startsWith('--')) {
      args[key] = 'true';
      continue;
    }

    args[key] = next;
    index += 1;
  }
  return args;
}

function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function severityRank(severity) {
  switch (severity) {
    case 'critical':
      return 5;
    case 'high':
      return 4;
    case 'moderate':
      return 3;
    case 'low':
      return 2;
    case 'info':
      return 1;
    default:
      return 0;
  }
}

function normalizeVulnerabilities(vulnerabilities) {
  return Object.entries(vulnerabilities || {})
    .map(([name, entry]) => ({ name, ...entry }))
    .sort((left, right) => {
      const severityDiff = severityRank(right.severity) - severityRank(left.severity);
      if (severityDiff !== 0) {
        return severityDiff;
      }
      return left.name.localeCompare(right.name);
    });
}

function renderFixAvailable(fixAvailable) {
  if (fixAvailable === true) {
    return 'available';
  }
  if (fixAvailable === false || fixAvailable == null) {
    return 'none';
  }
  if (typeof fixAvailable === 'object') {
    const name = fixAvailable.name || 'unknown-package';
    const version = fixAvailable.version || 'unknown-version';
    return `${name}@${version}`;
  }
  return String(fixAvailable);
}

function renderVia(via) {
  if (!Array.isArray(via) || via.length === 0) {
    return 'n/a';
  }

  return via.slice(0, 3).map((item) => {
    if (typeof item === 'string') {
      return item;
    }

    const source = item.source ? `#${item.source}` : (item.name || 'unknown');
    const severity = item.severity ? ` (${item.severity})` : '';
    return `${source}${severity}`;
  }).join(', ');
}

function buildReport(payload, project, registry, exitCode) {
  const lines = [];
  lines.push(`# npm Audit Summary: ${project}`);
  lines.push('');
  lines.push(`- Registry: \`${registry}\``);
  lines.push(`- Exit code: \`${exitCode}\``);

  if (payload && payload.metadata && payload.metadata.vulnerabilities) {
    const totals = payload.metadata.vulnerabilities;
    const deps = payload.metadata.dependencies || {};
    const vulnerabilities = normalizeVulnerabilities(payload.vulnerabilities);

    lines.push(`- Total vulnerabilities: \`${totals.total || 0}\``);
    lines.push(`- Severity counts: \`critical=${totals.critical || 0}\`, \`high=${totals.high || 0}\`, \`moderate=${totals.moderate || 0}\`, \`low=${totals.low || 0}\`, \`info=${totals.info || 0}\``);
    lines.push(`- Production dependencies audited: \`${deps.prod || 0}\``);
    lines.push('');

    if (vulnerabilities.length === 0) {
      lines.push('No production dependency vulnerabilities were reported at the selected audit level.');
      lines.push('');
      return lines.join('\n');
    }

    lines.push('## Vulnerabilities');
    lines.push('');
    for (const entry of vulnerabilities) {
      lines.push(`- ${entry.name}: severity=\`${entry.severity || 'unknown'}\`, range=\`${entry.range || 'unknown'}\`, direct=\`${entry.isDirect === true}\`, fix=\`${renderFixAvailable(entry.fixAvailable)}\`, via=${renderVia(entry.via)}`);
    }
    lines.push('');
    return lines.join('\n');
  }

  lines.push(`- Audit status: \`error\``);
  if (payload && payload.message) {
    lines.push(`- Error message: ${payload.message}`);
  }
  if (payload && payload.uri) {
    lines.push(`- Endpoint: \`${payload.uri}\``);
  }
  if (payload && payload.statusCode) {
    lines.push(`- Status code: \`${payload.statusCode}\``);
  }
  lines.push('');
  lines.push('Audit did not return the standard vulnerability payload. Check registry configuration or the uploaded artifact for full details.');
  lines.push('');
  return lines.join('\n');
}

function main() {
  const args = parseArgs(process.argv);
  const inputPath = args.input;
  const reportPath = args.report;
  const project = args.project || path.basename(process.cwd());
  const registry = args.registry || 'unknown';
  const exitCode = Number(args['exit-code'] || '0');

  if (!inputPath || !reportPath) {
    console.error('Usage: npm_audit_summary.js --input <json> --report <md> [--project <name>] [--registry <url>] [--exit-code <n>]');
    process.exit(1);
  }

  const payload = readJson(inputPath);
  const report = buildReport(payload, project, registry, exitCode);
  fs.mkdirSync(path.dirname(reportPath), { recursive: true });
  fs.writeFileSync(reportPath, report);
  process.stdout.write(report);
}

main();