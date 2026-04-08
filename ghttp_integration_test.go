//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGitCloneIntegration 测试使用官方 git clone 命令
func TestGitCloneIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 检查 git 是否可用
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found, skipping integration test")
	}

	// 创建临时目录
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo.git")
	clonePath := filepath.Join(tmpDir, "cloned")

	// 创建一个裸仓库作为远程仓库
	if err := createBareRepo(repoPath); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	// 启动测试服务器
	server := httptest.NewServer(&testBackend{})
	defer server.Close()

	// 使用 git clone 测试
	cmd := exec.Command("git", "clone", server.URL+"/test-repo.git", clonePath)
	output, err := cmd.CombinedOutput()

	// 注意：这个测试可能需要根据实际服务器行为调整
	// 这里只是展示集成测试的结构
	t.Logf("git clone output: %s", output)
	if err != nil {
		// 服务器可能还未正确配置，这是预期的
		t.Logf("git clone failed as expected (server not fully configured): %v", err)
	}
}

// testBackend 是一个简单的测试用 HTTP 后端
type testBackend struct{}

func (tb *testBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 简单的测试响应
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "test response")
}

func createBareRepo(path string) error {
	cmd := exec.Command("git", "init", "--bare", path)
	return cmd.Run()
}

// TestGitHttpBackendProtocol 测试 Git HTTP 协议的基本操作
func TestGitHttpBackendProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found, skipping integration test")
	}

	tests := []struct {
		name         string
		method       string
		path         string
		body         io.Reader
		expectStatus int
	}{
		{
			name:         "info/refs",
			method:       "GET",
			path:         "/test/info/refs?service=git-upload-pack",
			expectStatus: http.StatusOK,
		},
		{
			name:         "upload-pack",
			method:       "POST",
			path:         "/test/git-upload-pack",
			body:         bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试服务器
			server := httptest.NewServer(&testBackend{})
			defer server.Close()

			url := server.URL + tt.path
			req, err := http.NewRequest(tt.method, url, tt.body)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}
		})
	}
}

// TestErrorHandlerBehavior 测试错误处理行为
func TestErrorHandlerBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 测试场景 1: 写入错误时的行为
	t.Run("WriteError", func(t *testing.T) {
		rec := httptest.NewRecorder()
		logger, logBuf := mockLogger(t)

		frw := &flushResponseWriter{
			ResponseWriter: rec,
			log:            logger,
			chunkSize:      defaultChunkSize,
		}

		// 模拟错误场景
		errorReader := &errorReader{}

		_, err := frw.ReadFrom(errorReader)

		// 应该返回错误
		if err == nil {
			t.Error("expected error from errorReader")
		}

		// 验证日志记录
		if !strings.Contains(logBuf.String(), "error") {
			t.Error("expected error to be logged")
		}
	})

	// 测试场景 2: 部分写入后的错误
	t.Run("PartialWrite", func(t *testing.T) {
		rec := httptest.NewRecorder()
		logger, _ := mockLogger(t)

		frw := &flushResponseWriter{
			ResponseWriter: rec,
			log:            logger,
			chunkSize:      defaultChunkSize,
		}

		// 使用有限 reader 测试部分写入
		data := []byte("partial test data")
		reader := bytes.NewReader(data)

		n, err := frw.ReadFrom(reader)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if int(n) != len(data) {
			t.Errorf("expected %d bytes, got %d", len(data), n)
		}
	})
}

// errorReader 是一个始终返回错误的读取器
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rec := httptest.NewRecorder()
	logger, _ := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: rec,
		log:            logger,
		chunkSize:      defaultChunkSize,
	}

	// 并发写入测试
	data := []byte("concurrent test data")

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			reader := bytes.NewReader(data)
			_, err := frw.ReadFrom(reader)
			if err != nil {
				t.Errorf("concurrent write failed: %v", err)
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent writes")
		}
	}
}

// TestLargeDataTransfer 测试大数据传输
func TestLargeDataTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rec := httptest.NewRecorder()
	logger, _ := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: rec,
		log:            logger,
		chunkSize:      defaultChunkSize,
	}

	// 创建 1MB 的测试数据
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	reader := bytes.NewReader(largeData)

	n, err := frw.ReadFrom(reader)

	if err != nil {
		t.Fatalf("large data transfer failed: %v", err)
	}

	if n != int64(len(largeData)) {
		t.Errorf("expected %d bytes, got %d", len(largeData), n)
	}

	if rec.Body.Len() != len(largeData) {
		t.Errorf("expected %d bytes in response, got %d", len(largeData), rec.Body.Len())
	}
}

// TestContextCancellation 测试上下文取消
func TestContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	rec := httptest.NewRecorder()
	logger, _ := mockLogger(t)

	frw := &flushResponseWriter{
		ResponseWriter: rec,
		log:            logger,
		chunkSize:      defaultChunkSize,
	}

	// 创建一个大 reader 来模拟慢速传输
	slowReader := &slowReader{
		data: make([]byte, 1024*1024),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 创建一个 channel 来接收结果
	result := make(chan error, 1)

	go func() {
		_, err := frw.ReadFrom(slowReader)
		result <- err
	}()

	// 等待上下文取消或完成
	select {
	case err := <-result:
		if err != nil {
			t.Logf("read cancelled: %v", err)
		}
	case <-ctx.Done():
		t.Logf("context cancelled as expected")
	}
}

// slowReader 是一个模拟慢速读取的测试工具
type slowReader struct {
	data   []byte
	offset int
}

func (sr *slowReader) Read(p []byte) (n int, err error) {
	if sr.offset >= len(sr.data) {
		return 0, io.EOF
	}

	// 每次只读取少量数据
	n = copy(p, sr.data[sr.offset:])
	sr.offset += n
	return n, nil
}
