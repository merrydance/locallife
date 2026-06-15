package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/merrydance/locallife/internal/releasereadiness"
	"github.com/merrydance/locallife/util"
)

func main() {
	root := flag.String("root", ".", "backend repository root to scan")
	format := flag.String("format", "text", "output format: text or json")
	includeConfig := flag.Bool("include-config", false, "also load config from root and check production fail-fast readiness")
	includeRedis := flag.Bool("include-redis", false, "also ping Redis and read Asynq queue stats using loaded config")
	includeProviderClients := flag.Bool("include-provider-clients", false, "also construct provider clients using loaded config without making provider requests")
	flag.Parse()

	var config util.Config
	report, err := releasereadiness.Check(releasereadiness.Options{
		Root:         *root,
		Expectations: releasereadiness.DefaultExpectations(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "release readiness smoke failed:", err)
		os.Exit(2)
	}
	if *includeConfig || *includeRedis || *includeProviderClients {
		config, err = util.LoadConfig(*root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "release readiness config load failed:", err)
			os.Exit(2)
		}
	}
	if *includeConfig {
		report = releasereadiness.MergeReports(report, releasereadiness.CheckConfig(config))
	}
	if *includeRedis {
		report = releasereadiness.MergeReports(report, releasereadiness.CheckRedisAsynq(releasereadiness.RedisAsynqOptions{
			Address:        config.RedisAddress,
			Password:       config.RedisPassword,
			RequiredQueues: []string{"critical", "default"},
		}))
	}
	if *includeProviderClients {
		report = releasereadiness.MergeReports(report, releasereadiness.CheckBaofuProviderClients(config))
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "encode release readiness report:", err)
			os.Exit(2)
		}
	case "text", "":
		var sb strings.Builder
		releasereadiness.WriteText(report, &sb)
		fmt.Print(sb.String())
	default:
		fmt.Fprintln(os.Stderr, "unsupported format:", *format)
		os.Exit(2)
	}

	if report.Status != releasereadiness.StatusPass {
		os.Exit(1)
	}
}
