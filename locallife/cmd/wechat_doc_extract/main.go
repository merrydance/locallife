package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/merrydance/locallife/internal/wechatdoc"
)

func main() {
	docPath := flag.String("doc", "", "Path to the markdown document to extract")
	outPath := flag.String("out", "", "Optional JSON output path; defaults to stdout")
	flag.Parse()

	if *docPath == "" {
		fmt.Fprintln(os.Stderr, "missing required -doc flag")
		os.Exit(2)
	}

	result, err := wechatdoc.ExtractMarkdownFile(*docPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract wechat doc failed:", err)
		os.Exit(2)
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal extraction result failed:", err)
		os.Exit(2)
	}
	payload = append(payload, '\n')

	if *outPath == "" {
		if _, err := os.Stdout.Write(payload); err != nil {
			fmt.Fprintln(os.Stderr, "write extraction result failed:", err)
			os.Exit(2)
		}
	} else {
		if err := os.WriteFile(*outPath, payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write extraction output failed:", err)
			os.Exit(2)
		}
	}

	fmt.Fprintf(os.Stderr,
		"extracted sections=%d endpoints=%d fields=%d enum_sets=%d enum_values=%d error_codes=%d unknown_tables=%d warnings=%d\n",
		result.Summary.SectionCount,
		result.Summary.EndpointCount,
		result.Summary.FieldCount,
		result.Summary.EnumSetCount,
		result.Summary.EnumValueCount,
		result.Summary.ErrorCodeCount,
		result.Summary.UnknownTableCount,
		result.Summary.WarningCount,
	)
}
