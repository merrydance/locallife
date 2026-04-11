#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '..', '..');
const promptDir = path.join(repoRoot, '.github', 'prompts');
const promptReadmePath = path.join(promptDir, 'README.md');

function readFile(filePath) {
  return fs.readFileSync(filePath, 'utf8');
}

function parseFrontmatter(filePath) {
  const content = readFile(filePath);
  const lines = content.split(/\r?\n/);
  if (lines[0] !== '---') {
    throw new Error(`${path.relative(repoRoot, filePath)} is missing frontmatter`);
  }

  let endIndex = -1;
  for (let index = 1; index < lines.length; index += 1) {
    if (lines[index] === '---') {
      endIndex = index;
      break;
    }
  }

  if (endIndex === -1) {
    throw new Error(`${path.relative(repoRoot, filePath)} is missing closing frontmatter delimiter`);
  }

  const frontmatter = {};
  for (let index = 1; index < endIndex; index += 1) {
    const match = lines[index].match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (!match) {
      continue;
    }
    const rawValue = match[2].trim();
    frontmatter[match[1]] = rawValue.replace(/^['"]|['"]$/g, '');
  }

  return frontmatter;
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

function parseRoutingCases(readmeContent) {
  const lines = readmeContent.split(/\r?\n/);
  const cases = [];
  let inSection = false;

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (line === '## Routing Test Cases') {
      inSection = true;
      continue;
    }

    if (inSection && line.startsWith('## ')) {
      break;
    }

    if (!inSection) {
      continue;
    }

    const queryMatch = line.match(/^\d+\.\s+"(.+)"$/);
    if (!queryMatch) {
      continue;
    }

    const expectedLine = lines[index + 1] || '';
    const expectedMatch = expectedLine.match(/^Expected target:\s+`([^`]+)`$/);
    if (!expectedMatch) {
      throw new Error(`Routing test case '${queryMatch[1]}' is missing an Expected target line.`);
    }

    cases.push({ query: queryMatch[1], expected: expectedMatch[1] });
  }

  return cases;
}

function normalize(text) {
  return text.toLowerCase().replace(/\s+/g, ' ').trim();
}

function scorePrompt(query, hints) {
  const normalizedQuery = normalize(query);
  let score = 0;
  const matchedHints = [];

  for (const hint of hints) {
    const normalizedHint = normalize(hint);
    if (!normalizedHint) {
      continue;
    }
    if (normalizedQuery.includes(normalizedHint)) {
      score += normalizedHint.length;
      matchedHints.push(hint);
    }
  }

  return { score, matchedHints };
}

function main() {
  const promptFiles = fs.readdirSync(promptDir)
    .filter((name) => name.endsWith('.prompt.md'))
    .sort();

  const prompts = promptFiles.map((name) => {
    const filePath = path.join(promptDir, name);
    const frontmatter = parseFrontmatter(filePath);
    const description = frontmatter.description || '';
    const hints = extractTriggerPhrases(description);

    if (hints.length === 0) {
      throw new Error(`${path.relative(repoRoot, filePath)} description must declare Trigger phrases`);
    }

    if (Object.prototype.hasOwnProperty.call(frontmatter, 'routing-hints')) {
      throw new Error(`${path.relative(repoRoot, filePath)} uses unsupported frontmatter field routing-hints`);
    }

    return {
      fileName: name,
      hints
    };
  });

  const routingCases = parseRoutingCases(readFile(promptReadmePath));
  const failures = [];

  for (const routingCase of routingCases) {
    const scoredPrompts = prompts
      .map((prompt) => ({
        ...prompt,
        ...scorePrompt(routingCase.query, prompt.hints)
      }))
      .sort((left, right) => right.score - left.score || left.fileName.localeCompare(right.fileName));

    const best = scoredPrompts[0];
    const ties = scoredPrompts.filter((prompt) => prompt.score === best.score && prompt.score > 0);

    if (!best || best.score === 0) {
      failures.push(`No prompt matched routing case '${routingCase.query}'. Expected ${routingCase.expected}.`);
      continue;
    }

    if (ties.length > 1) {
      failures.push(
        `Ambiguous routing for '${routingCase.query}': ${ties.map((prompt) => prompt.fileName).join(', ')} all scored ${best.score}.`
      );
      continue;
    }

    if (best.fileName !== routingCase.expected) {
      failures.push(
        `Routing mismatch for '${routingCase.query}': expected ${routingCase.expected}, got ${best.fileName} via trigger phrases ${best.matchedHints.join(', ')}.`
      );
    }
  }

  if (failures.length > 0) {
    console.error('Prompt routing tests failed:');
    for (const failure of failures) {
      console.error(`- ${failure}`);
    }
    process.exit(1);
  }

  console.log('Prompt routing tests passed.');
  console.log(`- Routing cases: ${routingCases.length}`);
  console.log(`- Prompt files scored: ${prompts.length}`);
}

main();