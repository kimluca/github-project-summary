package main

import (
	"encoding/json"
	"fmt"
	"archaeologist/internal/analyzer"
)

func main() {
	report, err := analyzer.Analyze(".")
	if err != nil {
		panic(err)
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(b))
}
