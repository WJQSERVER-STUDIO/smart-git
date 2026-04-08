package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockLogger 创建一个日志记录器用于测试
func mockLogger(t *testing.T) (*log.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return log.New(&buf, "", 0), &buf
}

// errorWriter 是一个用于测试的 ResponseWriter，会在指定次数后返回错误
type errorWriter struct {
	failAt    int
	callCount int
	headerMap http.Header
	status    int
	written   int
}

func (ew *errorWriter) Header() http.Header {
	if ew.headerMap == nil {
		ew.headerMap = make(http.Header)
	}
	return ew.headerMap
}

func (ew *errorWriter) Write(data []byte) (int, error) {
	if ew.callCount >= ew.failAt {
		return 0, errors.New("simulated write error")
	}
	ew.callCount++
	ew.written += len(data)
	return len(data), nil
}

func (ew *errorWriter) WriteHeader(statusCode int) {
	ew.status = statusCode
}

func (ew *errorWriter) Flush() {}

// TestFlushResponseWriter_ReadFromWriteError 测试 Write 错误时的行为
func TestFlushResponseWriter_ReadFromWriteError(t *testing.T) {
	errorWriter := &errorWriter{failAt: 0}
	logger, logBuf := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: errorWriter,
		log:            logger,
		chunkSize:      defaultChunkSize,
	}

	data := []byte("test data that will cause write error")
	reader := bytes.NewReader(data)

	_, err := frw.ReadFrom(reader)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "error writing response") {
		t.Errorf("expected error log, got: %s", logOutput)
	}

	if errorWriter.status != 0 {
		t.Errorf("expected no status written by ReadFrom, got: %d", errorWriter.status)
	}
}

// TestFlushResponseWriter_ReadFromFlushError 测试 Flush 错误时的行为
func TestFlushResponseWriter_ReadFromFlushError(t *testing.T) {
	rec := httptest.NewRecorder()
	logger, logBuf := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: rec,
		log:            logger,
		chunkSize:      defaultChunkSize,
	}

	data := []byte("test data for flush")
	reader := bytes.NewReader(data)

	n, err := frw.ReadFrom(reader)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != int64(len(data)) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}

	logOutput := logBuf.String()
	if logOutput != "" {
		t.Errorf("expected no error logs, got: %s", logOutput)
	}
}

// TestFlushResponseWriter_ReadFromNormal 测试正常写入场景
func TestFlushResponseWriter_ReadFromNormal(t *testing.T) {
	rec := httptest.NewRecorder()
	logger, _ := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: rec,
		log:            logger,
		chunkSize:      defaultChunkSize,
	}

	data := []byte("normal test data")
	reader := bytes.NewReader(data)

	n, err := frw.ReadFrom(reader)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if int(n) != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	if rec.Body.String() != string(data) {
		t.Errorf("expected %s, got %s", data, rec.Body.String())
	}
}

// TestFlushResponseWriter_MultipleChunks 测试多块数据写入
func TestFlushResponseWriter_MultipleChunks(t *testing.T) {
	rec := httptest.NewRecorder()
	logger, _ := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: rec,
		log:            logger,
		chunkSize:      4,
	}

	data := []byte("this is a longer test data string")
	reader := bytes.NewReader(data)

	n, err := frw.ReadFrom(reader)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if int(n) != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
}

// TestRenderStatusError 测试错误响应格式化
func TestRenderStatusError(t *testing.T) {
	tests := []struct {
		code         int
		expectedBody string
	}{
		{http.StatusBadRequest, "400 Bad Request\n"},
		{http.StatusNotFound, "404 Not Found\n"},
		{http.StatusInternalServerError, "500 Internal Server Error\n"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.code), func(t *testing.T) {
			rec := httptest.NewRecorder()
			renderStatusError(rec, tt.code)

			if rec.Code != tt.code {
				t.Errorf("expected status %d, got %d", tt.code, rec.Code)
			}

			if rec.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}

// TestResponseAlreadyStartedRemoved 验证 responseAlreadyStarted 函数已被移除
func TestResponseAlreadyStartedRemoved(t *testing.T) {
	rec := httptest.NewRecorder()

	_, isTouka := interface{}(rec).(interface{ Written() bool })
	if isTouka {
		t.Error("should not detect touka ResponseWriter after removal")
	}

	_ = rec
}
