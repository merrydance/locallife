package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/util"
)

type regionRow struct {
	Code       string
	Name       string
	Level      int16
	ParentCode *string
	Longitude  *float64
	Latitude   *float64
}

func main() {
	var (
		configPath = flag.String("config", ".", "config path containing app.env")
		dbURL      = flag.String("db", "", "database connection string (default: DB_SOURCE from config)")
		dryRun     = flag.Bool("dry-run", false, "print actions without writing to DB")
		maxLevel   = flag.Int("max-level", 3, "max region level to sync (1-3 recommended)")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cfg, err := util.LoadConfig(*configPath)
	if err != nil {
		exitErr(fmt.Errorf("load config: %w", err))
	}

	if cfg.TencentMapKey == "" {
		exitErr(errors.New("TENCENT_MAP_KEY is empty (set it in app.env or env var)"))
	}

	connStr := strings.TrimSpace(*dbURL)
	if connStr == "" {
		connStr = strings.TrimSpace(cfg.DBSource)
	}
	if connStr == "" {
		exitErr(errors.New("db connection string is empty (pass -db or set DB_SOURCE)"))
	}

	if *maxLevel < 1 || *maxLevel > 3 {
		exitErr(fmt.Errorf("unsupported -max-level=%d (allowed 1..3)", *maxLevel))
	}

	client := maps.NewTencentMapClient(cfg.TencentMapKey)

	// 目标：河北全量到3级 + 广州到3级（含广东省父级）
	rows, err := buildTargetRegions(ctx, client, *maxLevel)
	if err != nil {
		exitErr(err)
	}

	fmt.Printf("准备同步 %d 条 regions 记录 (dry-run=%v)\n", len(rows), *dryRun)

	if *dryRun {
		printSummary(rows)
		return
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		exitErr(fmt.Errorf("connect db: %w", err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		exitErr(fmt.Errorf("ping db: %w", err))
	}

	if err := upsertRegions(ctx, pool, rows); err != nil {
		exitErr(err)
	}

	fmt.Println("✅ regions 同步完成")
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}

func buildTargetRegions(ctx context.Context, client *maps.TencentMapClient, maxLevel int) ([]regionRow, error) {
	levels, err := client.ListAllDistricts(ctx)
	if err != nil {
		return nil, fmt.Errorf("tencent district list all failed: %w", err)
	}

	rows := make([]regionRow, 0, 5000)

	// Level 1: Provinces
	if len(levels) >= 1 {
		for _, p := range levels[0] {
			rows = append(rows, districtToRow(p, 1, nil))
		}
	}

	// Level 2: Cities
	if maxLevel >= 2 && len(levels) >= 2 {
		for _, c := range levels[1] {
			// 腾讯 API 的 ID 前两位是省份代码
			parentCode := normalizeCode(c.ID)[:2] + "0000"
			rows = append(rows, districtToRow(c, 2, &parentCode))
		}
	}

	// Level 3: Districts
	if maxLevel >= 3 && len(levels) >= 3 {
		for _, d := range levels[2] {
			// 腾讯 API 的 ID 前四位是城市代码
			parentCode := normalizeCode(d.ID)[:4] + "00"
			rows = append(rows, districtToRow(d, 3, &parentCode))
		}
	}

	// 去重 + 稳定排序（父级优先）
	rows = dedupeRows(rows)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Level != rows[j].Level {
			return rows[i].Level < rows[j].Level
		}
		return rows[i].Code < rows[j].Code
	})

	return rows, nil
}

func districtToRow(d maps.District, level int16, parent *string) regionRow {
	name := d.FullName
	if strings.TrimSpace(name) == "" {
		name = d.Name
	}
	code := normalizeCode(d.ID)

	var lng, lat *float64
	if d.Location != nil {
		lng = &d.Location.Lng
		lat = &d.Location.Lat
	}

	return regionRow{
		Code:       code,
		Name:       name,
		Level:      level,
		ParentCode: parent,
		Longitude:  lng,
		Latitude:   lat,
	}
}

func normalizeCode(id string) string {
	id = strings.TrimSpace(id)
	// 有些场景可能出现数值字符串，统一为无前导/尾随空格
	if id == "" {
		return id
	}
	if n, err := strconv.ParseInt(id, 10, 64); err == nil {
		return fmt.Sprintf("%06d", n)
	}
	// 如果是纯数字但超过 int64 或包含前导零，这里保持原样
	return id
}

func dedupeRows(rows []regionRow) []regionRow {
	seen := map[string]regionRow{}
	order := make([]string, 0, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.Code]; !ok {
			order = append(order, r.Code)
		}
		// 后写覆盖（通常更完整的会在后面出现）
		seen[r.Code] = r
	}

	out := make([]regionRow, 0, len(seen))
	for _, code := range order {
		out = append(out, seen[code])
	}
	return out
}

func printSummary(rows []regionRow) {
	counts := map[int16]int{}
	for _, r := range rows {
		counts[r.Level]++
	}
	levels := make([]int, 0, len(counts))
	for lvl := range counts {
		levels = append(levels, int(lvl))
	}
	sort.Ints(levels)
	for _, lvl := range levels {
		fmt.Printf("level=%d count=%d\n", lvl, counts[int16(lvl)])
	}
}

func upsertRegions(ctx context.Context, pool *pgxpool.Pool, rows []regionRow) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	codeToID := map[string]int64{}

	const upsertSQL = `
INSERT INTO regions (code, name, level, parent_id, longitude, latitude)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  level = EXCLUDED.level,
  parent_id = EXCLUDED.parent_id,
  longitude = COALESCE(EXCLUDED.longitude, regions.longitude),
  latitude  = COALESCE(EXCLUDED.latitude,  regions.latitude)
RETURNING id;
`

	for _, r := range rows {
		var parentID any
		if r.ParentCode != nil {
			pid, ok := codeToID[*r.ParentCode]
			if !ok {
				return fmt.Errorf("parent not inserted yet: child=%s parent=%s", r.Code, *r.ParentCode)
			}
			parentID = pid
		}

		var lng any
		if r.Longitude != nil {
			lng = *r.Longitude
		}
		var lat any
		if r.Latitude != nil {
			lat = *r.Latitude
		}

		var id int64
		err := tx.QueryRow(ctx, upsertSQL, r.Code, r.Name, r.Level, parentID, lng, lat).Scan(&id)
		if err != nil {
			return fmt.Errorf("upsert region code=%s: %w", r.Code, err)
		}
		codeToID[r.Code] = id
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
