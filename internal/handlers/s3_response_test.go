package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestCopyS3ResponsePreservesRangeHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/video.mp4", nil)
	result := &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("data")), ContentLength: aws.Int64(4),
		ContentRange: aws.String("bytes 2-5/10"), ContentType: aws.String("video/mp4"), ETag: aws.String("etag"),
	}
	copyS3Response(recorder, request, result, "application/octet-stream")
	if recorder.Code != http.StatusPartialContent || recorder.Header().Get("Content-Range") != "bytes 2-5/10" || recorder.Body.String() != "data" {
		t.Fatalf("unexpected S3 response: code=%d headers=%v body=%q", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}
