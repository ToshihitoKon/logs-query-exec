package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

type LogsQueryExecRequest struct {
	LogGroupNames []string `json:"log_group_names"`
	QueryString   *string  `json:"query_string"`
	StartTime     *int64   `json:"start_time"`
	EndTime       *int64   `json:"end_time"`
	Limit         *int32   `json:"limit"`
}

func main() {
	var optStartTime = pflag.Int64("start", -1, "start time (unix timestamp)")
	var optEndTime = pflag.Int64("end", -1, "end time (unix timestamp)")
	var optLimit = pflag.Int32("limit", -1, "Result rows limit")
	var optQueryFile = pflag.String("query-file", "", "CloudWatch Logs Insights Query file")
	pflag.Parse()

	errors := []error{}
	if *optStartTime < 0 {
		errors = append(errors, fmt.Errorf("--start is required"))
	}
	if *optEndTime < 0 {
		errors = append(errors, fmt.Errorf("--end is required"))
	}
	if *optLimit < 0 {
		errors = append(errors, fmt.Errorf("--limit is required"))
	}
	if *optQueryFile == "" {
		errors = append(errors, fmt.Errorf("--query-file is required"))
	}

	if len(errors) != 0 {
		for _, err := range errors {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		os.Exit(1)
	}

	req := &LogsQueryExecRequest{
		StartTime: optStartTime,
		EndTime:   optEndTime,
		Limit:     optLimit,
	}

	data, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(string(data))

}
