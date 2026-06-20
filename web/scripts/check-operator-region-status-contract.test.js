(async () => {
  const fs = await import("node:fs");
  const path = await import("node:path");

  const repoRoot = path.resolve(__dirname, "..");

  function read(relativePath) {
    return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
  }

  function assert(condition, message) {
    if (!condition) {
      throw new Error(message);
    }
  }

  const types = read("src/types/operator-stats.ts");
  assert(
    /status:\s*"active"\s*\|\s*"suspended"\s*\|\s*string;/.test(types),
    "OperatorRegionResponse must expose backend operator region relation status"
  );

  const display = read("src/lib/operator-display.ts");
  assert(
    /formatOperatorRegionStatus/.test(display) &&
      /active:\s*"运营中"/.test(display) &&
      /suspended:\s*"已暂停"/.test(display),
    "Operator region status must be mapped to business-readable labels"
  );
  assert(
    /getActiveOperatorRegions/.test(display),
    "Operator action pages must share an active-region filter helper"
  );

  [
    "src/app/operator/peak-hours/page.tsx",
    "src/app/operator/regions/stats/page.tsx",
  ].forEach((relativePath) => {
    const source = read(relativePath);
    assert(
      !/\/operator\/regions",\s*\{\s*page:\s*1,\s*limit:\s*1\s*\}/.test(source),
      `${relativePath} must fetch enough operator regions before filtering active relations`
    );
  });

  [
    "src/app/operator/peak-hours/page.tsx",
    "src/app/operator/rules/page.tsx",
    "src/app/operator/merchants/manage/page.tsx",
    "src/app/operator/riders/manage/page.tsx",
    "src/app/operator/regions/stats/page.tsx",
  ].forEach((relativePath) => {
    const source = read(relativePath);
    assert(
      /getActiveOperatorRegions/.test(source),
      `${relativePath} must filter operator regions to active relations before operational use`
    );
  });

  const regionsPage = read("src/app/operator/regions/page.tsx");
  assert(
    /formatOperatorRegionStatus/.test(regionsPage),
    "Operator region management page must display relation status labels"
  );
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
