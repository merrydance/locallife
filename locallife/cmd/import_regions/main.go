package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type RegionData struct {
	ID        int64    `json:"id"`
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Level     int16    `json:"level"`
	ParentID  *int64   `json:"parent_id"`
	Longitude *float64 `json:"longitude"`
	Latitude  *float64 `json:"latitude"`
}

type RegionsFile struct {
	Regions []RegionData `json:"regions"`
}

func main() {
	// 连接数据库
	conn, err := pgx.Connect(context.Background(), "postgresql:///locallife_test?sslmode=disable")
	if err != nil {
		log.Fatal("无法连接数据库:", err)
	}
	defer conn.Close(context.Background())

	// 读取JSON文件
	data, err := os.ReadFile("regions_202511242305.json")
	if err != nil {
		log.Fatal("无法读取文件:", err)
	}

	var regionsFile RegionsFile
	if err := json.Unmarshal(data, &regionsFile); err != nil {
		log.Fatal("无法解析JSON:", err)
	}

	fmt.Printf("准备导入 %d 条区域数据...\n", len(regionsFile.Regions))

	// 批量插入
	batchSize := 1000
	for i := 0; i < len(regionsFile.Regions); i += batchSize {
		end := i + batchSize
		if end > len(regionsFile.Regions) {
			end = len(regionsFile.Regions)
		}
		batch := regionsFile.Regions[i:end]

		// 使用CopyFrom批量插入
		rows := make([][]interface{}, len(batch))
		for j, r := range batch {
			var parentID pgtype.Int8
			if r.ParentID != nil {
				parentID = pgtype.Int8{Int64: *r.ParentID, Valid: true}
			}

			var longitude, latitude pgtype.Numeric
			if r.Longitude != nil {
				longitude = pgtype.Numeric{Valid: true}
			}
			if r.Latitude != nil {
				latitude = pgtype.Numeric{Valid: true}
			}

			rows[j] = []interface{}{r.ID, r.Code, r.Name, r.Level, parentID, longitude, latitude}
		}

		_, err := conn.CopyFrom(
			context.Background(),
			pgx.Identifier{"regions"},
			[]string{"id", "code", "name", "level", "parent_id", "longitude", "latitude"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			log.Printf("批量插入失败: %v\n", err)
			continue
		}

		fmt.Printf("已导入 %d / %d\n", end, len(regionsFile.Regions))
	}

	fmt.Println("导入完成！")
}
