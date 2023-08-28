package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	lqe "github.com/ToshihitoKon/logs-query-exec/src"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	ctx := context.Background()
	cli, err := lqe.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	conf, err := lqe.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	handler := getLambdaHandler(cli, conf)
	onLambda := strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda_") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
	if onLambda {
		lambda.Start(handler)
	} else {
		fmt.Fprintf(os.Stderr, "Execute from outside Lambda. Load sample request from file %s\n", conf.SampleRequestJson)

		payload, err := lqe.LoadLambdaPayloadSample(conf.SampleRequestJson)
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

func getLambdaHandler(cli *lqe.Client, conf *lqe.Config) func(context.Context, *lqe.RequestEvent) (*lqe.LogsQueryExecResponse, error) {
	return func(ctx context.Context, event *lqe.RequestEvent) (*lqe.LogsQueryExecResponse, error) {
		req := &lqe.LogsQueryExecRequest{}
		res := &lqe.LogsQueryExecResponse{}
		res.Status = lqe.ResponseStatusFailed

		log.Println(event.Body)
		body, err := event.GetBody()
		if err != nil {
			res.Error = fmt.Sprintf("error: get request body. %s", err.Error())
			return res, err
		}
		log.Println(body)

		if err := json.Unmarshal([]byte(body), req); err != nil {
			res.Error = fmt.Sprintf("error: json.Unmarshal request body. %s", err.Error())
			return res, err
		}
		log.Printf("%#v", req)
		if errors := req.Validate(); len(errors) != 0 {
			for _, v := range errors {
				fmt.Fprintln(os.Stderr, v.Error())
			}
			res.Error = fmt.Sprintf("Bad Request")
			return res, fmt.Errorf("Bad Request")
		}

		queryId, result, err := cli.RunQuery(ctx, conf, req)
		res.QueryId = queryId
		if err != nil {
			if errors.Is(err, lqe.ErrorEnableRetry) {
				res.EnableRetry = true
				return res, nil
			}
			res.Error = fmt.Sprintf("failed runQuery. %s", err.Error())
			return res, err
		}

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

		if err := cli.S3Copy(ctx, conf, bytes.NewReader(result), queryId+".json"); err != nil {
			res.Error = fmt.Sprintf("error upload s3. %s", err.Error())
			return res, err
		}

		res.FileName = queryId + ".json"
		res.FilePath = path.Join(conf.Aws.S3Bucket, conf.Aws.S3ObjectKeyPrefix, res.FileName)
		res.Status = lqe.ResponseStatusSuccess

		return res, nil
	}
}
