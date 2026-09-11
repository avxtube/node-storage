package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func copyS3Response(w http.ResponseWriter, r *http.Request, result *s3.GetObjectOutput, fallbackMime string) {
	contentType := fallbackMime
	if result.ContentType != nil && *result.ContentType != "" {
		contentType = *result.ContentType
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	if result.ContentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*result.ContentLength, 10))
	}
	if result.ETag != nil {
		w.Header().Set("ETag", *result.ETag)
	}
	if result.ContentRange != nil {
		w.Header().Set("Content-Range", *result.ContentRange)
		w.WriteHeader(http.StatusPartialContent)
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, result.Body)
}
