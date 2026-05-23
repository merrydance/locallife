#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '..', '..');
const githubRoot = path.join(repoRoot, '.github');
const promptDir = path.join(githubRoot, 'prompts');
const agentDir = path.join(githubRoot, 'agents');

function readFile(filePath) {
  return fs.readFileSync(filePath, 'utf8');
}

function walkMarkdownFiles(dirPath) {
  const results = [];
  for (const entry of fs.readdirSync(dirPath, { withFileTypes: true })) {
    const fullPath = path.join(dirPath, entry.name);
    if (entry.isDirectory()) {
      results.push(...walkMarkdownFiles(fullPath));
      continue;
    }
    if (entry.isFile() && entry.name.endsWith('.md')) {
      results.push(fullPath);
    }
  }
  return results.sort();
}

function normalizeQuotedValue(value) {
  const trimmed = value.trim();
  if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function parseFrontmatter(filePath) {
  const content = readFile(filePath);
  const lines = content.split(/\r?\n/);

  if (lines[0] !== '---') {
    return { errors: ['missing opening frontmatter delimiter'], frontmatter: {}, body: content };
  }

  let endIndex = -1;
  for (let index = 1; index < lines.length; index += 1) {
    if (lines[index] === '---') {
      endIndex = index;
      break;
    }
  }

  if (endIndex === -1) {
    return { errors: ['missing closing frontmatter delimiter'], frontmatter: {}, body: content };
  }

  const frontmatter = {};
  const errors = [];

  for (let index = 1; index < endIndex; index += 1) {
    const line = lines[index];
    if (!line.trim()) {
      continue;
    }

    const match = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (!match) {
      errors.push(`invalid frontmatter line: ${line}`);
      continue;
    }

    frontmatter[match[1]] = normalizeQuotedValue(match[2]);
  }

  return {
    errors,
    frontmatter,
    body: lines.slice(endIndex + 1).join('\n')
  };
}

function parseCurrentTemplates(indexContent) {
  const lines = indexContent.split(/\r?\n/);
  const templates = [];
  let inSection = false;

  for (const line of lines) {
    if (line.startsWith('## ')) {
      if (line === '## Current Templates') {
        inSection = true;
        continue;
      }
      if (inSection) {
        break;
      }
    }

    if (!inSection) {
      continue;
    }

    const match = line.match(/^- `([^`]+\.prompt\.md)`$/);
    if (match) {
      templates.push(match[1]);
    }
  }

  return templates;
}

function extractTriggerPhrases(description) {
  const marker = 'Trigger phrases:';
  const markerIndex = description.indexOf(marker);
  if (markerIndex === -1) {
    return [];
  }

  const afterMarker = description.slice(markerIndex + marker.length);
  const sentenceEnd = afterMarker.search(/[。.]/);
  const phraseBlock = sentenceEnd === -1 ? afterMarker : afterMarker.slice(0, sentenceEnd);
  return phraseBlock
    .split(',')
    .map((phrase) => phrase.trim().toLowerCase())
    .map((phrase) => phrase.replace(/[;，]+$/g, ''))
    .filter(Boolean);
}

function collectRepoReferences(filePath, content) {
  const references = [];
  const patterns = [
    /`((?:\.github|weapp|web|locallife|artifacts)\/[^`]+)`/g,
    /(?<![A-Za-z0-9_./-])((?:\.github|weapp|web|locallife|artifacts)\/[A-Za-z0-9_./\-]+)(?![A-Za-z0-9_./-])/g
  ];

  for (const pattern of patterns) {
    let match;
    while ((match = pattern.exec(content)) !== null) {
      const nextCharacter = content[match.index + match[1].length] || '';
      const ref = match[1].replace(/[.,:;)]*$/, '');
      if (
        !ref ||
        ref.includes('*') ||
        ref.includes('{') ||
        ref.includes('}') ||
        ref.includes('<') ||
        ref.includes('>') ||
        nextCharacter === '*'
      ) {
        continue;
      }
      references.push({ source: path.relative(repoRoot, filePath), ref });
    }
  }

  return references;
}

function unique(values) {
  return [...new Set(values)];
}

function lintFileContains(errors, relativePath, snippets, messageSuffix) {
  const filePath = path.join(repoRoot, relativePath);
  const content = readFile(filePath);

  for (const snippet of snippets) {
    if (!content.includes(snippet)) {
      errors.push(`${relativePath}: ${messageSuffix} '${snippet}'`);
    }
  }
}

function lintFileContainsCaseInsensitive(errors, relativePath, snippets, messageSuffix) {
  const filePath = path.join(repoRoot, relativePath);
  const content = readFile(filePath).toLowerCase();

  for (const snippet of snippets) {
    if (!content.includes(snippet.toLowerCase())) {
      errors.push(`${relativePath}: ${messageSuffix} '${snippet}'`);
    }
  }
}

function lintFileOmits(errors, relativePath, snippets, messageSuffix) {
  const filePath = path.join(repoRoot, relativePath);
  const content = readFile(filePath);

  for (const snippet of snippets) {
    if (content.includes(snippet)) {
      errors.push(`${relativePath}: ${messageSuffix} '${snippet}'`);
    }
  }
}

function lintFileConditionalContains(errors, relativePath, triggerSnippet, requiredSnippets, messageSuffix) {
  const filePath = path.join(repoRoot, relativePath);
  const content = readFile(filePath);

  if (!content.includes(triggerSnippet)) {
    return;
  }

  for (const snippet of requiredSnippets) {
    if (!content.includes(snippet)) {
      errors.push(`${relativePath}: ${messageSuffix} '${snippet}'`);
    }
  }
}

function lintBackendCanonicalOwners(errors) {
  const backendEntryFiles = [
    '.github/copilot-instructions.md',
    '.github/instructions/backend-locallife.instructions.md',
    'locallife/AGENTS.md'
  ];

  for (const relativePath of backendEntryFiles) {
    lintFileContains(
      errors,
      relativePath,
      ['.github/standards/backend/README.md'],
      'backend entrypoint must reference canonical backend index'
    );
  }

  lintFileContains(
    errors,
    'locallife/AGENTS.md',
    ['../.github/prompts/'],
    'backend agent guide must keep .github as the canonical prompt entrypoint'
  );

  lintFileConditionalContains(
    errors,
    'locallife/AGENTS.md',
    '.codex/prompts/',
    ['backend-local wrappers', 'not the long-term source of truth'],
    'backend agent guide must clearly downgrade legacy .codex prompt wrappers when they are mentioned'
  );

  const codexPromptOwners = {
    'locallife/.codex/prompts/feature.md': ['Compatibility Pointer', '.github/prompts/backend-implementation.prompt.md'],
    'locallife/.codex/prompts/bugfix.md': ['Compatibility Pointer', '.github/prompts/backend-bugfix.prompt.md'],
    'locallife/.codex/prompts/review.md': ['Compatibility Pointer', '.github/prompts/backend-review-closure.prompt.md', '.github/review/open-findings.md', '.github/review/audit-log.md'],
    'locallife/.codex/prompts/full-audit.md': ['Compatibility Pointer', '.github/prompts/backend-review-closure.prompt.md', '.github/review/open-findings.md', '.github/review/audit-log.md'],
    'locallife/.codex/prompts/takeover.md': ['Compatibility Pointer', '.github/prompts/backend-takeover.prompt.md'],
    'locallife/.codex/prompts/schema-change.md': ['Compatibility Pointer', '.github/standards/backend/SQL_STANDARDS.md', '.github/prompts/backend-sql-review.prompt.md']
  };

  for (const [relativePath, snippets] of Object.entries(codexPromptOwners)) {
    lintFileContains(errors, relativePath, snippets, 'legacy codex prompt wrapper is missing canonical pointer');
  }

  const codexContextOwners = {
    'locallife/.codex/context/architecture.md': ['Compatibility Pointer', '.github/standards/backend/RUNTIME_ARCHITECTURE.md'],
    'locallife/.codex/context/workflow.md': ['Compatibility Pointer', '.github/standards/backend/WORKFLOW_AND_VALIDATION.md'],
    'locallife/.codex/context/risk-map.md': ['Compatibility Pointer', '.github/standards/backend/README.md'],
    'locallife/.codex/context/review-loop.md': ['Compatibility Pointer', '.github/standards/backend/FORMAL_REVIEW_DURABILITY.md', '.github/review/open-findings.md', '.github/review/audit-log.md']
  };

  for (const [relativePath, snippets] of Object.entries(codexContextOwners)) {
    lintFileContains(errors, relativePath, snippets, 'legacy codex context wrapper is missing canonical pointer');
  }

  const codexChecklistOwners = {
    'locallife/.codex/checklists/change-safety.md': [
      'Compatibility Pointer',
      '.github/standards/backend/BACKEND_CHANGE_SAFETY_CHECKLIST.md',
      'compatibility pointer',
      'canonical `.github` checklist'
    ],
    'locallife/.codex/checklists/review-closeout.md': [
      'Compatibility Pointer',
      '.github/standards/backend/BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md',
      '.github/standards/backend/FORMAL_REVIEW_DURABILITY.md',
      '.github/review/open-findings.md',
      '.github/review/audit-log.md',
      'compatibility pointer',
      'canonical `.github` checklist'
    ]
  };

  for (const [relativePath, snippets] of Object.entries(codexChecklistOwners)) {
    lintFileContains(errors, relativePath, snippets, 'legacy codex checklist is missing canonical pointer');
  }

  lintFileOmits(
    errors,
    'locallife/.codex/checklists/change-safety.md',
    ['## Path Coverage', '## Invariants', '## Generated Outputs', '## Tests', '## Workspace Safety'],
    'legacy change-safety checklist must stay a thin compatibility pointer instead of restoring duplicated sections'
  );

  lintFileOmits(
    errors,
    'locallife/.codex/checklists/review-closeout.md',
    ['## Findings', '## Durable Knowledge', '## Audit Trail', '## Verification', '## Closure'],
    'legacy review-closeout checklist must stay a thin compatibility pointer instead of restoring duplicated sections'
  );

  const codexReviewOwners = {
    'locallife/.codex/review/open-findings.md': ['Compatibility Pointer', '.github/review/open-findings.md'],
    'locallife/.codex/review/audit-log.md': ['Compatibility Pointer', '.github/review/audit-log.md']
  };

  for (const [relativePath, snippets] of Object.entries(codexReviewOwners)) {
    lintFileContains(errors, relativePath, snippets, 'legacy codex review ledger is missing canonical pointer');
  }

  lintFileContains(
    errors,
    '.github/standards/backend/FORMAL_REVIEW_DURABILITY.md',
    ['.github/review/open-findings.md', '.github/review/audit-log.md'],
    'formal review durability doc must reference canonical review ledgers'
  );

  for (const relativePath of [
    '.github/standards/backend/FORMAL_REVIEW_DURABILITY.md',
    '.github/instructions/review.instructions.md',
    'locallife/AGENTS.md'
  ]) {
    lintFileOmits(
      errors,
      relativePath,
      ['.codex/review/'],
      'active review owner must not drift back to legacy .codex review ledgers via'
    );
  }
}

function lintWeappPromptBoundaries(errors) {
  lintFileContains(
    errors,
    '.github/instructions/weapp-mini-program.instructions.md',
    [
      '.github/standards/weapp/PAGE_DELIVERY_BASELINE.md',
      'TDesign MCP',
      '.github/prompts/weapp-implementation.prompt.md',
      '.github/prompts/weapp-review.prompt.md'
    ],
    'weapp hot-path instruction is missing canonical weapp prompt or standards anchor'
  );

  lintFileContains(
    errors,
    '.github/prompts/weapp-implementation.prompt.md',
    [
      '.github/standards/weapp/README.md',
      '.github/standards/weapp/PAGE_DELIVERY_BASELINE.md',
      '.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md',
      '.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md',
      'State the task risk level',
      'State which relevant paths remain unverified',
      'Payment-related mode:'
    ],
    'weapp implementation prompt is missing required risk or canonical standards anchor'
  );

  lintFileOmits(
    errors,
    '.github/prompts/weapp-implementation.prompt.md',
    ['Delivery baseline:'],
    'weapp implementation prompt must stay prompt-shaped instead of restoring a standards-like body via'
  );

  lintFileContains(
    errors,
    '.github/prompts/weapp-review.prompt.md',
    [
      '.github/standards/weapp/README.md',
      '.github/standards/weapp/PAGE_DELIVERY_BASELINE.md',
      '.github/standards/weapp/REVIEW_CHECKLIST.md',
      '.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md',
      '.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md',
      'Infer or confirm the task risk level',
      'Overall upgrade audit add-on:',
      'Payment / high-risk review add-on:'
    ],
    'weapp review prompt is missing required risk or review-mode boundary anchor'
  );

  lintFileOmits(
    errors,
    '.github/prompts/weapp-review.prompt.md',
    [
      'Popup forms use a stable bottom action area instead of leaving action buttons inside scroll content tails',
      'Bottom popup dual actions render as equal-width block buttons and do not degrade into content-width small buttons'
    ],
    'weapp review prompt should rely on canonical weapp standards for low-level UI rules instead of restating'
  );
}

function lintContextRehydrationGate(errors) {
  const fileRequirements = {
    '.github/README.md': ['rerun routing from `.github/README.md`', 'Do not keep relying on stale context.'],
    '.github/copilot-instructions.md': ['rerun routing from `.github/README.md`', 'Do not keep relying on stale context.'],
    '.github/standards/engineering/AI_PROMPT_GOVERNANCE.md': ['rerun routing from `.github/README.md`', 'Do not keep relying on stale context.'],
    'locallife/AGENTS.md': ['rerun routing from `../.github/README.md`', 'Do not keep relying on stale context.']
  };

  for (const [relativePath, snippets] of Object.entries(fileRequirements)) {
    lintFileContainsCaseInsensitive(
      errors,
      relativePath,
      snippets,
      'context rehydration gate must be declared in the canonical entrypoints'
    );
  }

  const gateSnippets = [
    'new, compacted, forked, or handed off',
    'rerun routing from',
    'reopen the matching instructions',
    'Do not keep relying on stale context.'
  ];

  const promptAndInstructionFiles = [
    ...fs.readdirSync(promptDir).filter((name) => name.endsWith('.prompt.md')).map((name) => path.join(promptDir, name)),
    ...walkMarkdownFiles(path.join(repoRoot, '.github', 'instructions'))
  ];

  for (const filePath of promptAndInstructionFiles) {
    const relativePath = path.relative(repoRoot, filePath);
    lintFileContainsCaseInsensitive(errors, relativePath, gateSnippets, 'prompt or instruction must keep the context rehydration gate');
  }
}

function main() {
  const errors = [];
  const promptFiles = fs.readdirSync(promptDir)
    .filter((name) => name.endsWith('.prompt.md'))
    .sort()
    .map((name) => path.join(promptDir, name));
  const agentFiles = fs.readdirSync(agentDir)
    .filter((name) => name.endsWith('.agent.md'))
    .sort()
    .map((name) => path.join(agentDir, name));

  const seenPromptNames = new Map();
  const seenDescriptions = new Map();
  const seenTriggerPhrases = new Map();
  const agentNames = new Map();

  for (const agentFile of agentFiles) {
    const parsed = parseFrontmatter(agentFile);
    for (const error of parsed.errors) {
      errors.push(`${path.relative(repoRoot, agentFile)}: ${error}`);
    }

    const agentName = parsed.frontmatter.name;
    if (!agentName) {
      errors.push(`${path.relative(repoRoot, agentFile)}: missing frontmatter field 'name'`);
      continue;
    }
    agentNames.set(agentName, path.basename(agentFile));
  }

  for (const promptFile of promptFiles) {
    const parsed = parseFrontmatter(promptFile);
    const relativePath = path.relative(repoRoot, promptFile);
    for (const error of parsed.errors) {
      errors.push(`${relativePath}: ${error}`);
    }

    const promptName = parsed.frontmatter.name;
    const description = parsed.frontmatter.description;

    if (!promptName) {
      errors.push(`${relativePath}: missing frontmatter field 'name'`);
    }
    if (!description) {
      errors.push(`${relativePath}: missing frontmatter field 'description'`);
    }
    if (Object.prototype.hasOwnProperty.call(parsed.frontmatter, 'routing-hints')) {
      errors.push(`${relativePath}: unsupported frontmatter field 'routing-hints'; keep routing tokens in description 'Trigger phrases:' instead`);
    }

    if (promptName) {
      if (seenPromptNames.has(promptName)) {
        errors.push(`${relativePath}: duplicate prompt name '${promptName}' also used by ${seenPromptNames.get(promptName)}`);
      } else {
        seenPromptNames.set(promptName, relativePath);
      }
    }

    if (description) {
      if (seenDescriptions.has(description)) {
        errors.push(`${relativePath}: duplicate prompt description also used by ${seenDescriptions.get(description)}`);
      } else {
        seenDescriptions.set(description, relativePath);
      }

      if (!description.includes('Trigger phrases:')) {
        errors.push(`${relativePath}: description must declare 'Trigger phrases:'`);
      }

      for (const phrase of extractTriggerPhrases(description)) {
        if (seenTriggerPhrases.has(phrase)) {
          errors.push(`${relativePath}: trigger phrase '${phrase}' also appears in ${seenTriggerPhrases.get(phrase)}`);
        } else {
          seenTriggerPhrases.set(phrase, relativePath);
        }
      }
    }

    if (parsed.frontmatter.agent && !agentNames.has(parsed.frontmatter.agent)) {
      errors.push(`${relativePath}: referenced agent '${parsed.frontmatter.agent}' does not exist in .github/agents/*.agent.md`);
    }

    const baseName = path.basename(promptFile);
    if (baseName === 'general-implementation.prompt.md' || baseName === 'general-review.prompt.md') {
      if (!description || !description.includes('Use only when')) {
        errors.push(`${relativePath}: general routing boundary must keep 'Use only when' in description`);
      }
      if (!parsed.body.includes('Use `general-')) {
        errors.push(`${relativePath}: body must explain when to defer to area-specific prompts`);
      }
    }
  }

  const promptIndexPath = path.join(promptDir, 'README.md');
  const promptIndexContent = readFile(promptIndexPath);
  const indexedTemplates = parseCurrentTemplates(promptIndexContent);
  const promptBaseNames = promptFiles.map((filePath) => path.basename(filePath));

  for (const promptName of promptBaseNames) {
    if (!indexedTemplates.includes(promptName)) {
      errors.push(`.github/prompts/README.md: prompt '${promptName}' is not listed under 'Current Templates'`);
    }
  }

  for (const template of indexedTemplates) {
    if (!promptBaseNames.includes(template)) {
      errors.push(`.github/prompts/README.md: indexed prompt '${template}' does not exist`);
    }
  }

  const agentIndexContent = readFile(path.join(agentDir, 'README.md'));
  for (const agentFile of agentFiles) {
    const agentBaseName = path.basename(agentFile);
    if (!agentIndexContent.includes(`\`${agentBaseName}\``)) {
      errors.push(`.github/agents/README.md: agent '${agentBaseName}' is not mentioned in the agent index`);
    }
  }

  const aiFacingFiles = unique([
    path.join(githubRoot, 'README.md'),
    path.join(githubRoot, 'copilot-instructions.md'),
    path.join(promptDir, 'README.md'),
    path.join(agentDir, 'README.md'),
    ...walkMarkdownFiles(path.join(githubRoot, 'review')),
    ...walkMarkdownFiles(path.join(githubRoot, 'instructions')),
    ...walkMarkdownFiles(promptDir),
    ...walkMarkdownFiles(agentDir),
    ...walkMarkdownFiles(path.join(githubRoot, 'standards', 'engineering')),
    ...walkMarkdownFiles(path.join(githubRoot, 'standards', 'backend')),
    path.join(repoRoot, 'locallife', 'AGENTS.md'),
    ...walkMarkdownFiles(path.join(repoRoot, 'locallife', '.codex', 'context')),
    ...walkMarkdownFiles(path.join(repoRoot, 'locallife', '.codex', 'prompts')),
    ...walkMarkdownFiles(path.join(repoRoot, 'locallife', '.codex', 'checklists')),
    ...walkMarkdownFiles(path.join(repoRoot, 'locallife', '.codex', 'review'))
  ]);

  lintBackendCanonicalOwners(errors);
  lintWeappPromptBoundaries(errors);
  lintContextRehydrationGate(errors);

  const seenReferences = new Set();
  for (const filePath of aiFacingFiles) {
    const content = readFile(filePath);
    for (const reference of collectRepoReferences(filePath, content)) {
      const dedupeKey = `${reference.source}::${reference.ref}`;
      if (seenReferences.has(dedupeKey)) {
        continue;
      }
      seenReferences.add(dedupeKey);

      const targetPath = path.join(repoRoot, reference.ref);
      if (!fs.existsSync(targetPath)) {
        errors.push(`${reference.source}: referenced path '${reference.ref}' does not exist`);
      }
    }
  }

  if (errors.length > 0) {
    console.error('Prompt governance lint failed:');
    for (const error of errors) {
      console.error(`- ${error}`);
    }
    process.exit(1);
  }

  console.log(`Prompt governance lint passed.`);
  console.log(`- Prompt files: ${promptFiles.length}`);
  console.log(`- Agent files: ${agentFiles.length}`);
  console.log(`- Indexed templates: ${indexedTemplates.length}`);
  console.log(`- AI-facing markdown files checked: ${aiFacingFiles.length}`);
}

main();
