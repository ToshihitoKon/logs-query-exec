# logs-query-exec

CloudWatch Logs Insightsを叩いて結果をファイル出力し、指定したコマンドを実行するGoのヤツ


request

```
{
    body: (Base64Encoded:Optional) {
        logGroupNames: ["log group names", ...],
        encodedQueryString: "base64 encoded CloudWatch Logs Insights query",
        startTime: "Start unixtimestamp",
        endTime: "End unixtimestamp",
        limit: "Query result limit",
    },
    IsBase64Encoded: Boolean
}
```
