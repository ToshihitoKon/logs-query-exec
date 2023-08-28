package logsQueryExec

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cwl "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/shogo82148/go-retry"
)

type Client struct {
	AwsConfig aws.Config
	CwlClient *cwl.Client
	S3Client  *s3.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	awsConf, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	cwlCli := cwl.NewFromConfig(awsConf)
	s3Cli := s3.NewFromConfig(awsConf)

	return &Client{
		AwsConfig: awsConf,
		CwlClient: cwlCli,
		S3Client:  s3Cli,
	}, nil
}

func (cli *Client) cwlQueryStart(ctx context.Context, req *LogsQueryExecRequest) (string, error) {
	queryData, err := base64.StdEncoding.DecodeString(*req.EncodedQueryString)
	if err != nil {
		return "", err
	}
	queryStr := string(queryData)

	params := &cwl.StartQueryInput{
		LogGroupNames: req.LogGroupNames,
		QueryString:   &queryStr,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		Limit:         req.Limit,
	}

	res, err := cli.CwlClient.StartQuery(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed StartQuery: %w", err)
	}
	return *res.QueryId, nil
}

func (cli *Client) cwlGetQueryResultWithRetry(ctx context.Context, config *Config, params *cwl.GetQueryResultsInput) (logEntries, error) {
	policy := retry.Policy{
		MinDelay: 300 * time.Millisecond,
		MaxDelay: 15 * time.Second,
		MaxCount: config.Retry.MaxCount,
	}

	var res *cwl.GetQueryResultsOutput
	var err error

	retrier := policy.Start(ctx)
LOOP:
	for retrier.Continue() {
		res, err = cli.CwlClient.GetQueryResults(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("failed GetQueryResults: %w", err)
		}
		switch res.Status {
		case types.QueryStatusComplete:
			break LOOP
		case types.QueryStatusScheduled, types.QueryStatusRunning:
			log.Printf("info: GetQueryResults returned sutatus: %s: %w", res.Status, ErrorEnableRetry)
			continue
		default:
			return nil, fmt.Errorf("error: GetQueryResults returned sutatus: %s", res.Status)
		}
	}

	if res.Status != types.QueryStatusComplete {
		return nil, fmt.Errorf("error: GetQueryResults expire retry limit: %w", ErrorEnableRetry)
	}

	entries := logEntries{}
	for _, entry := range res.Results {
		fieldMap := logEntry{}
		for _, field := range entry {
			fieldMap[*field.Field] = *field.Value
		}
		entries = append(entries, fieldMap)
	}
	return entries, nil
}

func (cli *Client) RunQuery(ctx context.Context, config *Config, req *LogsQueryExecRequest) (string, []byte, error) {
	var queryId string
	if req.QueryId != nil {
		queryId = *req.QueryId
	}

	var err error

	if queryId == "" {
		queryId, err = cli.cwlQueryStart(ctx, req)
		if err != nil {
			return "", nil, fmt.Errorf("cwlfailed QueryStart: %w", err)
		}
	}

	resultsParams := &cwl.GetQueryResultsInput{
		QueryId: &queryId,
	}

	results, err := cli.cwlGetQueryResultWithRetry(ctx, config, resultsParams)
	if err != nil {
		return queryId, nil, fmt.Errorf("failed cwlGetQueryResultWithRetry: %w", err)
	}

	jsonResByte, err := json.Marshal(results)
	if err != nil {
		return queryId, nil, fmt.Errorf("failed json.Marshal: %w", err)
	}

	return queryId, jsonResByte, nil
}

func (cli *Client) S3Copy(ctx context.Context, config *Config, body io.Reader, dest string) error {
	bucket := config.Aws.S3Bucket
	prefix := config.Aws.S3ObjectKeyPrefix
	key := path.Join(prefix, dest)

	_, err := cli.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Body:   body,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("failed PutObject: %w", err)
	}
	return nil
}
