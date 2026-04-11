package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

type collectorNode struct {
	Code      string
	Name      string
	Level     int16
	Parent    *string
	Longitude *float64
	Latitude  *float64
	Children  []map[string]any
}

func main() {
	var (
		configPath  = flag.String("config", ".", "config path containing app.env")
		dbURL       = flag.String("db", "", "database connection string (default: DB_SOURCE from config)")
		dryRun      = flag.Bool("dry-run", false, "print actions without writing to DB")
		seedKeyword = flag.String("seed", "中国", "seed keyword for administrative lookup")
		endpoint    = flag.String("endpoint", "/administrative", "tianditu administrative endpoint path")
		needAll     = flag.Bool("need-all", true, "request all levels from seed query")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cfg, err := util.LoadConfig(*configPath)
	if err != nil {
		exitErr(fmt.Errorf("load config: %w", err))
	}

	if strings.TrimSpace(cfg.TiandituMapKey) == "" {
		exitErr(errors.New("TIANDITU_MAP_KEY is empty (set it in app.env or env var)"))
	}

	connStr := strings.TrimSpace(*dbURL)
	if connStr == "" {
		connStr = strings.TrimSpace(cfg.DBSource)
	}
	if connStr == "" {
		exitErr(errors.New("db connection string is empty (pass -db or set DB_SOURCE)"))
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.TiandituBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.tianditu.gov.cn"
	}

	client := &tiandituDistrictClient{
		key:        cfg.TiandituMapKey,
		baseURL:    baseURL,
		endpoint:   normalizeEndpoint(*endpoint),
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}

	rows, err := client.BuildRegionRows(ctx, *seedKeyword, *needAll)
	if err != nil {
		exitErr(err)
	}

	fmt.Printf("准备同步 %d 条 regions 记录（天地图，dry-run=%v）\n", len(rows), *dryRun)
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

	fmt.Println("✅ 天地图 regions 同步完成")
}

type tiandituDistrictClient struct {
	key        string
	baseURL    string
	endpoint   string
	httpClient *http.Client
}

func (c *tiandituDistrictClient) BuildRegionRows(ctx context.Context, seed string, needAll bool) ([]regionRow, error) {
	payload := map[string]any{
		"searchWord":  seed,
		"searchType":  1,
		"needSubInfo": true,
		"needAll":     needAll,
	}

	resp, err := c.query(ctx, payload)
	if err != nil {
		return nil, err
	}

	forest, err := extractDistrictForest(resp)
	if err != nil {
		return nil, err
	}

	rows := make([]regionRow, 0, 5000)
	seen := make(map[string]struct{})
	for _, node := range forest {
		collectRows(node, nil, 0, &rows, seen)
	}

	rows = dedupeRows(rows)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Level != rows[j].Level {
			return rows[i].Level < rows[j].Level
		}
		return rows[i].Code < rows[j].Code
	})

	if len(rows) == 0 {
		return nil, errors.New("tianditu response parsed but no valid region rows produced")
	}
	return rows, nil
}

func (c *tiandituDistrictClient) query(ctx context.Context, post map[string]any) (map[string]any, error) {
	postJSON, err := json.Marshal(post)
	if err != nil {
		return nil, fmt.Errorf("marshal postStr: %w", err)
	}

	params := url.Values{}
	params.Set("tk", c.key)
	params.Set("postStr", string(postJSON))

	reqURL := c.baseURL + c.endpoint + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.tianditu.gov.cn/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request tianditu administrative api failed: %w", err)
	}
	defer resp.Body.Close()

	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode tianditu response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tianditu administrative http %d", resp.StatusCode)
	}

	status := getString(decoded, "status", "code")
	if status != "" && status != "0" && status != "200" {
		return nil, fmt.Errorf("tianditu administrative business status=%s msg=%s", status, getString(decoded, "msg", "message"))
	}

	return decoded, nil
}

func extractDistrictForest(resp map[string]any) ([]map[string]any, error) {
	candidates := []any{
		resp["data"],
		resp["result"],
		resp["returnData"],
		resp["administrative"],
		resp,
	}

	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if forest := findNodeObjects(candidate); len(forest) > 0 {
			return forest, nil
		}
	}

	return nil, errors.New("unable to locate district node array in tianditu response")
}

func findNodeObjects(value any) []map[string]any {
	switch node := value.(type) {
	case []any:
		out := make([]map[string]any, 0)
		for _, item := range node {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if looksLikeDistrictNode(obj) {
				out = append(out, obj)
			}
		}
		if len(out) > 0 {
			return out
		}
		for _, item := range node {
			if nested := findNodeObjects(item); len(nested) > 0 {
				return nested
			}
		}
	case map[string]any:
		if arr := tryGetArray(node, "all", "districts", "children", "child", "subDistrict", "data", "result", "items", "list"); len(arr) > 0 {
			if nested := findNodeObjects(arr); len(nested) > 0 {
				return nested
			}
		}
		for _, child := range node {
			if nested := findNodeObjects(child); len(nested) > 0 {
				return nested
			}
		}
	}
	return nil
}

func collectRows(node map[string]any, parentCode *string, depth int, out *[]regionRow, seen map[string]struct{}) {
	parsed := parseCollectorNode(node, parentCode, depth)
	if parsed.Code != "" && parsed.Name != "" && parsed.Level >= 1 && parsed.Level <= 3 {
		if _, ok := seen[parsed.Code]; !ok {
			seen[parsed.Code] = struct{}{}
			row := regionRow{
				Code:       parsed.Code,
				Name:       parsed.Name,
				Level:      parsed.Level,
				ParentCode: parsed.Parent,
				Longitude:  parsed.Longitude,
				Latitude:   parsed.Latitude,
			}
			*out = append(*out, row)
		}
	}

	nextParent := parentCode
	if parsed.Code != "" && parsed.Level >= 1 && parsed.Level <= 3 {
		cp := parsed.Code
		nextParent = &cp
	}

	for _, child := range parsed.Children {
		collectRows(child, nextParent, depth+1, out, seen)
	}
}

func parseCollectorNode(node map[string]any, parentCode *string, depth int) collectorNode {
	name := getString(node, "name", "districtName", "fullName", "fullname", "lName", "label")
	code := normalizeCode(getString(node, "code", "adcode", "cityCode", "adminCode", "id", "value"))
	if code == "" && name != "" {
		code = syntheticCode(name, depth)
	}

	level := int16(0)
	if lv, ok := getInt(node, "level", "adminLevel", "grade"); ok {
		if lv >= 1 && lv <= 3 {
			level = int16(lv)
		}
	}
	if level == 0 {
		level = int16(depth)
	}

	lng := getFloatPtr(node, "lon", "lng", "longitude", "x")
	lat := getFloatPtr(node, "lat", "latitude", "y")

	children := make([]map[string]any, 0)
	for _, key := range []string{"children", "child", "subDistrict", "districts", "items", "list"} {
		arr := tryGetArray(node, key)
		for _, item := range arr {
			obj, ok := item.(map[string]any)
			if ok {
				children = append(children, obj)
			}
		}
	}

	return collectorNode{
		Code:      code,
		Name:      name,
		Level:     level,
		Parent:    parentCode,
		Longitude: lng,
		Latitude:  lat,
		Children:  children,
	}
}

func looksLikeDistrictNode(node map[string]any) bool {
	name := getString(node, "name", "districtName", "fullName", "fullname", "lName", "label")
	code := getString(node, "code", "adcode", "cityCode", "adminCode", "id", "value")
	return strings.TrimSpace(name) != "" || strings.TrimSpace(code) != ""
}

func tryGetArray(node map[string]any, keys ...string) []any {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			if arr, ok := value.([]any); ok {
				return arr
			}
		}
	}
	return nil
}

func getString(node map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := node[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case json.Number:
			return typed.String()
		case float64:
			return strconv.FormatInt(int64(typed), 10)
		case int:
			return strconv.Itoa(typed)
		case int64:
			return strconv.FormatInt(typed, 10)
		}
	}
	return ""
}

func getInt(node map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := node[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed), true
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed), true
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func getFloatPtr(node map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := node[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			v := typed
			return &v
		case int:
			v := float64(typed)
			return &v
		case int64:
			v := float64(typed)
			return &v
		case json.Number:
			if parsed, err := typed.Float64(); err == nil {
				v := parsed
				return &v
			}
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				v := parsed
				return &v
			}
		}
	}
	return nil
}

func syntheticCode(name string, depth int) string {
	seed := fmt.Sprintf("%s-%d", strings.TrimSpace(name), depth)
	h := 0
	for _, r := range seed {
		h = (h*31 + int(r)) % 1000000
	}
	return fmt.Sprintf("%06d", h)
}

func normalizeCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return code
	}
	if n, err := strconv.ParseInt(code, 10, 64); err == nil {
		return fmt.Sprintf("%06d", n)
	}
	return code
}

func normalizeEndpoint(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return "/administrative"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	return clean
}

func dedupeRows(rows []regionRow) []regionRow {
	seen := map[string]regionRow{}
	order := make([]string, 0, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.Code]; !ok {
			order = append(order, r.Code)
		}
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

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}
