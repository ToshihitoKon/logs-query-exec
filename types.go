package main

import "encoding/json"

// 関数URLで叩いた場合は API Gateway V2 のペイロードに従うので、互換がある形にする
// ref: https://docs.aws.amazon.com/ja_jp/lambda/latest/dg/urls-invocation.html
type RequestEvent struct {
	Body string `json:"body"`
}

type LogsQueryExecRequest struct {
	LogGroupNames []string `json:"log_group_names"`
	QueryString   *string  `json:"query_string"`
	StartTime     *int64   `json:"start_time"`
	EndTime       *int64   `json:"end_time"`
	Limit         *int32   `json:"limit"`
}

func (req *LogsQueryExecRequest) Validate() []error {
	errors := []error{}
	if err := checkEmpty(req.LogGroupNames, "log_group_name"); err != nil {
		errors = append(errors, err)
	}
	if err := checkEmpty(req.QueryString, "query_string"); err != nil {
		errors = append(errors, err)
	}
	if err := checkEmpty(req.StartTime, "start_time"); err != nil {
		errors = append(errors, err)
	}
	if err := checkEmpty(req.EndTime, "end_time"); err != nil {
		errors = append(errors, err)
	}
	if err := checkEmpty(req.Limit, "limit"); err != nil {
		errors = append(errors, err)
	}

	return errors
}

type LogsQueryExecResponse struct {
	Status   string `json:"status"`
	QueryId  string `json:"query_id"`
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
}

// json.Marshalに失敗しても必ずjson stringを返す
func (res *LogsQueryExecResponse) toMustJson() string {
	bytesData, err := json.Marshal(res)
	if err != nil {
		return res.constantFailedJson()
	}
	return string(bytesData)
}

func (res *LogsQueryExecResponse) constantFailedJson() string {
	return `{"status":"failed"}`
}

type logEntry map[string]string
type logEntries []logEntry
