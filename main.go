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
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type LogsQueryExecRequest struct {
	LogGroupNames []string `json:"log_group_names"`
	QueryString   *string  `json:"query_string"`
	StartTime     *int64   `json:"start_time"`
	EndTime       *int64   `json:"end_time"`
	Limit         *int32   `json:"limit"`
}

func (req *LogsQueryExecRequest) Validate() error {
	log.Printf("%#v\n", req)
	if len(req.LogGroupNames) < 1 {
		return fmt.Errorf("Bad Request")
	}

	if req.QueryString == nil ||
		req.StartTime == nil ||
		req.EndTime == nil ||
		req.Limit == nil {
		return fmt.Errorf("Bad Request")
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

	onLambda := strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda_") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
	if onLambda {
		handler := getLambdaHandler(cli)
		lambda.Start(handler)
	} else {
		fmt.Println("Execute from outside Lambda")
		// handler := getOutsideLambdaHandler(cli)
		// if err := handler(); err != nil {
		// 	log.Fatal(err)
		// }

		// json読んでlambdaHandlerをテストする
		payload, err := loadLambdaPayloadSample()
		if err != nil {
			log.Fatal(err)
		}

		handler := getLambdaHandler(cli)
		result, err := handler(ctx, payload)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(result)
	}
}

func loadLambdaPayloadSample() (*events.APIGatewayV2HTTPRequest, error) {
	f, err := os.Open("sample_request.json")
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	req := &events.APIGatewayV2HTTPRequest{}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, err
	}

	return req, nil
}

// 関数URLで叩いた場合は API Gateway V2 のペイロードに従う
// ref: https://docs.aws.amazon.com/ja_jp/lambda/latest/dg/urls-invocation.html
func getLambdaHandler(cli *Client) func(context.Context, *events.APIGatewayV2HTTPRequest) (string, error) {
	return func(ctx context.Context, event *events.APIGatewayV2HTTPRequest) (string, error) {
		req := &LogsQueryExecRequest{}
		if err := json.Unmarshal([]byte(event.Body), req); err != nil {
			return "", err
		}
		if err := req.Validate(); err != nil {
			return "", err
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
