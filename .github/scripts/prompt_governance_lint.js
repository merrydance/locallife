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
    ...walkMarkdownFiles(path.join(githubRoot, 'instructions')),
    ...walkMarkdownFiles(promptDir),
    ...walkMarkdownFiles(agentDir),
    ...walkMarkdownFiles(path.join(githubRoot, 'standards', 'engineering'))
  ]);

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