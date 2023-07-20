package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

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
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		response, err := handler(ctx, payload)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Printf("%+v", response)
			os.Exit(1)
		}

		bytesData, err := json.Marshal(response)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Println(string(bytesData))
	}
}

func getLambdaHandler(cli *Client) func(context.Context, *RequestEvent) (*LogsQueryExecResponse, error) {
	return func(ctx context.Context, event *RequestEvent) (*LogsQueryExecResponse, error) {
		req := &LogsQueryExecRequest{}
		res := &LogsQueryExecResponse{}
		res.Status = ResponseStatusFailed

		log.Println(event.Body)
		body, err := event.getBody()
		if err != nil {
			res.Error = fmt.Sprintf("error: get request body. %s", err.Error())
			return res, err
		}

		if err := json.Unmarshal([]byte(body), req); err != nil {
			res.Error = fmt.Sprintf("error: json.Unmarshal request body. %s", err.Error())
			return res, err
		}
		if errors := req.Validate(); len(errors) != 0 {
			for _, v := range errors {
				fmt.Fprintln(os.Stderr, v.Error())
			}
			res.Error = fmt.Sprintf("Bad Request")
			return res, fmt.Errorf("Bad Request")
		}

		queryId, result, err := cli.runQuery(ctx, req)
		if err != nil {
			res.Error = fmt.Sprintf("failed runQuery. %s", err.Error())
			return res, err
		}
		res.QueryId = queryId

		filename := path.Join("/tmp", queryId+".json")
		f, err := os.Create(filename)
		if err != nil {
			res.Error = fmt.Sprintf("error os.Create. %s", err.Error())
			return res, err
		}

		defer func() {
			f.Close()
			os.Remove(filename)
		}()

		if _, err := f.Write(result); err != nil {
			res.Error = fmt.Sprintf("error file.Write. %s", err.Error())
			return res, err
		}

		if err := cli.s3Copy(ctx, bytes.NewReader(result), queryId+".json"); err != nil {
			res.Error = fmt.Sprintf("error upload s3. %s", err.Error())
			return res, err
		}

		res.FileName = queryId + ".json"
		res.FilePath = path.Join(lqeConfig.Aws.S3Bucket, lqeConfig.Aws.S3ObjectKeyPrefix, res.FileName)
		res.Status = ResponseStatusSuccess

		return res, nil
	}
}
