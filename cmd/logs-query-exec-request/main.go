package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	lqe "github.com/ToshihitoKon/logs-query-exec/src"
	"github.com/spf13/pflag"
)

type CliOption struct {
	StartTime     int64
	EndTime       int64
	Limit         int32
	LogGroupNames []string
	QueryFile     string
	OutFilename   string
}

func getCliOption() (*CliOption, []error) {
	var optLogGroupNames = pflag.StringSliceP("log-group-names", "g", []string{}, "Log group names (required)")
	var optStartTime = pflag.Int64P("start", "s", -1, "start time (unix timestamp) (required)")
	var optEndTime = pflag.Int64P("end", "e", -1, "end time (unix timestamp) (required)")
	var optLimit = pflag.Int32P("limit", "l", -1, "Result rows limit (required)")
	var optQueryFile = pflag.StringP("query-file", "q", "", "CloudWatch Logs Insights Query file (required)")
	var optOutFilename = pflag.StringP("out", "o", "", "Output file name. if not given, output stdout.")
	pflag.Parse()

	errors := []error{}
	if len(*optLogGroupNames) == 0 {
		errors = append(errors, fmt.Errorf("--log-group-names is required"))
	}
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
		return nil, errors
	}

	return &CliOption{
		StartTime:     *optStartTime,
		EndTime:       *optEndTime,
		Limit:         *optLimit,
		LogGroupNames: *optLogGroupNames,
		QueryFile:     *optQueryFile,
		OutFilename:   *optOutFilename,
	}, nil
}

func main() {
	opt, errs := getCliOption()
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	queryFile, err := os.Open(opt.QueryFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	var outFile *os.File
	if opt.OutFilename != "" {
		var err error
		outFile, err = os.Create(opt.OutFilename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	} else {
		outFile = os.Stdout
	}

	queryData, err := ioutil.ReadAll(queryFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	encodedQueryStr := string(base64.StdEncoding.EncodeToString(queryData))

	req := &lqe.LogsQueryExecRequest{
		LogGroupNames:      opt.LogGroupNames,
		StartTime:          &opt.StartTime,
		EndTime:            &opt.EndTime,
		Limit:              &opt.Limit,
		EncodedQueryString: &encodedQueryStr,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	encodedReqStr := base64.StdEncoding.EncodeToString(reqData)

	reqEvent := &lqe.RequestEvent{
		Body:            encodedReqStr,
		IsBase64Encoded: true,
	}

	reqEventData, err := json.Marshal(reqEvent)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Fprintf(outFile, string(reqEventData))
}
