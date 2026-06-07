const assert = require('assert')
const fs = require('fs')
const path = require('path')

const weappRoot = path.join(__dirname, '..')
const workspaceRoot = path.join(weappRoot, '..')

function readFromWeapp(relativePath) {
  return fs.readFileSync(path.join(weappRoot, relativePath), 'utf8')
}

function readFromWorkspace(relativePath) {
  return fs.readFileSync(path.join(workspaceRoot, relativePath), 'utf8')
}

const apiSource = readFromWeapp('miniprogram/api/merchant.ts')
const pageSource = readFromWeapp('miniprogram/pages/merchant/settings/membership/index.ts')
const backendRequestSource = readFromWorkspace('locallife/api/membership.go')
const backendSceneSource = readFromWorkspace('locallife/logic/membership_balance_scenes.go')
const alignmentMigrationSource = readFromWorkspace('locallife/db/migration/000252_align_membership_settings_scenes.up.sql')
const hardeningMigrationSource = readFromWorkspace('locallife/db/migration/000253_harden_membership_settings_scene_constraints.up.sql')
const hardeningMigrationDownSource = readFromWorkspace('locallife/db/migration/000253_harden_membership_settings_scene_constraints.down.sql')
const migrationSources = [alignmentMigrationSource, hardeningMigrationSource]

const sceneTypeMatch = apiSource.match(/export type MerchantMembershipScene = ([^\n]+)/)
assert(sceneTypeMatch, 'Mini Program membership scene type must be declared')
assert(
  sceneTypeMatch[1] === "'dine_in' | 'takeaway'",
  'Mini Program membership scene type must expose only backend-supported dine_in/takeaway'
)

for (const unsupportedScene of ['takeout', 'reservation']) {
  assert(
    !pageSource.includes(`key: '${unsupportedScene}'`),
    `merchant membership settings page must not expose unsupported ${unsupportedScene} scene`
  )
  assert(
    !pageSource.includes(`${unsupportedScene}: selectedSet.has('${unsupportedScene}')`),
    `merchant membership settings draft state must not preserve unsupported ${unsupportedScene} scene`
  )
}

for (const supportedScene of ['dine_in', 'takeaway']) {
  assert(
    pageSource.includes(`key: '${supportedScene}'`),
    `merchant membership settings page must expose supported ${supportedScene} scene`
  )
  assert(
    pageSource.includes(`${supportedScene}: selectedSet.has('${supportedScene}')`),
    `merchant membership settings draft state must initialize supported ${supportedScene} scene`
  )
}

assert(
  backendRequestSource.includes('oneof=dine_in takeaway'),
  'backend membership settings request binding must continue accepting dine_in/takeaway scenes'
)
assert(
  backendSceneSource.includes('[]string{"dine_in", "takeaway"}'),
  'backend membership balance supported scene list must continue using dine_in/takeaway'
)
assert(
  migrationSources.every((source) => source.includes("ALTER COLUMN balance_usable_scenes SET DEFAULT ARRAY['dine_in', 'takeaway']::TEXT[]")),
  'membership settings migration must align the DB default with dine_in/takeaway'
)
assert(
  migrationSources.every((source) => (
    source.includes("array_position(balance_usable_scenes, NULL) IS NULL") &&
    source.includes("balance_usable_scenes <@ ARRAY['dine_in', 'takeaway']::TEXT[]") &&
    source.includes("array_position(bonus_usable_scenes, NULL) IS NULL") &&
    source.includes("bonus_usable_scenes <@ ARRAY['dine_in', 'takeaway']::TEXT[]")
  )),
  'membership settings migration must reject unsupported or null stored scenes'
)
assert(
  migrationSources.every((source) => (
    source.includes('array_position(balance_usable_scenes, NULL) IS NOT NULL') &&
    source.includes('array_position(bonus_usable_scenes, NULL) IS NOT NULL')
  )),
  'membership settings migration must clean historical null scene values before adding CHECK constraints'
)
assert(
  hardeningMigrationSource.includes('DROP CONSTRAINT IF EXISTS merchant_membership_settings_balance_scenes_check') &&
    hardeningMigrationSource.includes('DROP CONSTRAINT IF EXISTS merchant_membership_settings_bonus_scenes_check'),
  'membership settings hardening migration must be safe for environments where 000252 already added constraints'
)
assert(
  hardeningMigrationDownSource.includes("ALTER COLUMN balance_usable_scenes SET DEFAULT ARRAY['dine_in', 'takeaway']::TEXT[]") &&
    !hardeningMigrationDownSource.includes("ARRAY['dine_in', 'takeout', 'reservation']::TEXT[]"),
  'membership settings hardening down migration must return to the 000252 contract, not the historical 000030 contract'
)

console.log('check-merchant-membership-settings-scenes: membership scene contract is aligned')
