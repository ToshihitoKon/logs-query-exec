package logsQueryExec

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cwl "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
		return "", err
	}
	return *res.QueryId, nil
}

func (cli *Client) cwlGetQueryResult(ctx context.Context, params *cwl.GetQueryResultsInput) (logEntries, error) {
	retryCount := 0
	var res *cwl.GetQueryResultsOutput
	var err error

Loop:
	for retryCount < 5 {
		res, err = cli.CwlClient.GetQueryResults(ctx, params)
		if err != nil {
			return nil, err
		}
		switch res.Status {
		case types.QueryStatusComplete:
			break Loop
		case types.QueryStatusScheduled, types.QueryStatusRunning:
			retryCount++
			randTime := rand.Intn(500)
			sleepTime := int64(math.Pow(2, float64(retryCount))*300) + int64(randTime)
			fmt.Fprintf(os.Stderr, "retry...(%d:%d)\n", retryCount, sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
			continue
		default:
			return nil, fmt.Errorf("error: GetQueryResults returned sutatus: %s", res.Status)
		}
	}

	if res.Status != types.QueryStatusComplete {
		return nil, fmt.Errorf("error: GetQueryResults returned sutatus: %s", res.Status)
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

func (cli *Client) RunQuery(ctx context.Context, req *LogsQueryExecRequest) (string, []byte, error) {
	queryId, err := cli.cwlQueryStart(ctx, req)
	if err != nil {
		return "", nil, err
	}

	resultsParams := &cwl.GetQueryResultsInput{
		QueryId: &queryId,
	}

	results, err := cli.cwlGetQueryResult(ctx, resultsParams)
	if err != nil {
		return queryId, nil, err
	}

	jsonResByte, err := json.Marshal(results)
	if err != nil {
		return queryId, nil, err
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
		return err
	}
	return nil
}
