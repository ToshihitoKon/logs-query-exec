package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

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

func checkEmpty(v any, varName string) error {
	log.Println(v)
	err := fmt.Errorf("%s is required", varName)
	val := reflect.ValueOf(v)

	switch val.Kind() {
	case reflect.Pointer:
		if v == nil || val.IsNil() {
			return err
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if val.Len() < 1 {
			return err
		}
	}

	return nil
}

type logEntry map[string]string
type logEntries []logEntry

func main() {
	ctx := context.Background()
	cli, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	handler := getLambdaHandler(cli)
	onLambda := strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda_") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
	if onLambda {
		lambda.Start(handler)
	} else {
		fmt.Fprintf(os.Stderr, "Execute from outside Lambda. Load sample request from file %s\n", lqeConfig.SampleRequestJson)

		payload, err := loadLambdaPayloadSample(lqeConfig.SampleRequestJson)
		if err != nil {
			log.Fatal(err)
		}

		result, err := handler(ctx, payload)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(result)
	}
}

func loadLambdaPayloadSample(filePath string) (*RequestEvent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	req := &RequestEvent{}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, err
	}

	return req, nil
}

func getLambdaHandler(cli *Client) func(context.Context, *RequestEvent) (string, error) {
	return func(ctx context.Context, event *RequestEvent) (string, error) {
		req := &LogsQueryExecRequest{}
		if err := json.Unmarshal([]byte(event.Body), req); err != nil {
			return "", err
		}
		if errors := req.Validate(); len(errors) != 0 {
			for _, v := range errors {
				fmt.Fprintln(os.Stderr, v.Error())
			}
			return "Bad Request", nil
		}

		queryId, result, err := cli.runQuery(ctx, req)
		if err != nil {
			return "", err
		}
		log.Println("queryId", queryId)

		filename := path.Join("/tmp", queryId+".json")
		f, err := os.Create(filename)
		if err != nil {
			return "", err
		}
		log.Println("filename", filename)

		defer func() {
			f.Close()
			os.Remove(filename)
		}()

		if _, err := f.Write(result); err != nil {
			return "", err
		}

		log.Println("s3Copy")
		if err := cli.s3Copy(ctx, bytes.NewReader(result), queryId+".json"); err != nil {
			return "", err
		}

		return queryId + ".json", nil
	}
}

func getOutsideLambdaHandler(cli *Client) func(*LogsQueryExecRequest) error {
	return func(req *LogsQueryExecRequest) error {
		ctx := context.Background()
		queryId, result, err := cli.runQuery(ctx, req)
		if err != nil {
			return err
		}

		f, err := os.Create(path.Join("tmp", "logs-query-exec", queryId+".json"))
		if err != nil {
			return err
		}

		n, err := f.Write(result)
		if err != nil {
			return err
		}

		fmt.Printf("saved %d bytes in %s", n, f.Name())

		return nil
	}
}
