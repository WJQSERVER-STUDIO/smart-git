//go:build behavior
// +build behavior

package main

import (
	"bytes"
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

// TestGitBehaviorCloneEmptyRepo 测试克隆空仓库的行为
func TestGitBehaviorCloneEmptyRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping behavior test in short mode")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found, skipping behavior test")
	}

	tmpDir := t.TempDir()
	clonePath := filepath.Join(tmpDir, "cloned")

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟空仓库响应
		if strings.HasSuffix(r.URL.Path, "/info/refs") {
			w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 执行 git clone
	cmd := exec.Command(gitPath, "clone", server.URL+"/test.git", clonePath)
	output, err := cmd.CombinedOutput()

	t.Logf("git clone output: %s", output)

	// 空仓库应该失败或成功，取决于服务器实现
	// 这里主要验证 git 命令能正常执行
	if err != nil {
		t.Logf("clone failed (may be expected for empty repo): %v", err)
	}
}

// TestGitBehaviorPushPull 测试 push 和 pull 的基本行为
func TestGitBehaviorPushPull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping behavior test in short mode")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found, skipping behavior test")
	}

	tmpDir := t.TempDir()
	localRepo := filepath.Join(tmpDir, "local")
	remoteRepo := filepath.Join(tmpDir, "remote.git")

	// 创建本地仓库
	if err := os.MkdirAll(localRepo, 0755); err != nil {
		t.Fatalf("failed to create local repo: %v", err)
	}

	// 初始化本地仓库
	runGit(t, gitPath, localRepo, "init")
	runGit(t, gitPath, localRepo, "config", "user.email", "test@example.com")
	runGit(t, gitPath, localRepo, "config", "user.name", "Test User")

	// 创建测试文件
	testFile := filepath.Join(localRepo, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 提交
	runGit(t, gitPath, localRepo, "add", "test.txt")
	runGit(t, gitPath, localRepo, "commit", "-m", "Initial commit")

	// 创建裸仓库作为远程
	runGit(t, gitPath, tmpDir, "init", "--bare", "remote.git")

	// 推送
	runGit(t, gitPath, localRepo, "push", remoteRepo, "master")

	// 克隆到另一个目录
	clonePath := filepath.Join(tmpDir, "cloned")
	runGit(t, gitPath, tmpDir, "clone", remoteRepo, "cloned")

	// 验证文件存在
	clonedFile := filepath.Join(clonePath, "test.txt")
	if _, err := os.Stat(clonedFile); err != nil {
		t.Errorf("cloned file not found: %v", err)
	}

	content, err := os.ReadFile(clonedFile)
	if err != nil {
		t.Errorf("failed to read cloned file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("unexpected content: %s", content)
	}
}

// TestGitBehaviorErrorHandling 测试错误处理行为
func TestGitBehaviorErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping behavior test in short mode")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found, skipping behavior test")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// 创建仓库
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	runGit(t, gitPath, repoPath, "init")
	runGit(t, gitPath, repoPath, "config", "user.email", "test@example.com")
	runGit(t, gitPath, repoPath, "config", "user.name", "Test User")

	// 尝试推送到不存在的远程 - 应该失败
	cmd := exec.Command(gitPath, "push", "http://invalid-hostname-12345/test.git", "master")
	output, err := cmd.CombinedOutput()

	t.Logf("push to invalid host output: %s", output)

	// 应该失败
	if err == nil {
		t.Error("expected push to fail")
	}
}

// TestGitBehaviorLargeFile 测试大文件处理
func TestGitBehaviorLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping behavior test in short mode")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found, skipping behavior test")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	remoteRepo := filepath.Join(tmpDir, "remote.git")

	// 创建仓库
	runGit(t, gitPath, tmpDir, "init", "repo")
	runGit(t, gitPath, repoPath, "config", "user.email", "test@example.com")
	runGit(t, gitPath, repoPath, "config", "user.name", "Test User")

	// 创建一个大文件 (1MB)
	largeFile := filepath.Join(repoPath, "large.bin")
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}

	// 写入 1MB 数据
	size := 1024 * 1024
	buf := make([]byte, 1024)
	for i := 0; i < size/1024; i++ {
		if _, err := f.Write(buf); err != nil {
			t.Fatalf("failed to write large file: %v", err)
		}
	}
	f.Close()

	runGit(t, gitPath, repoPath, "add", "large.bin")
	runGit(t, gitPath, repoPath, "commit", "-m", "Add large file")

	// 创建裸仓库
	runGit(t, gitPath, tmpDir, "init", "--bare", "remote.git")

	// 推送
	runGit(t, gitPath, repoPath, "push", remoteRepo, "master")

	// 克隆验证
	clonePath := filepath.Join(tmpDir, "cloned")
	runGit(t, gitPath, tmpDir, "clone", remoteRepo, "cloned")

	clonedFile := filepath.Join(clonePath, "large.bin")
	stat, err := os.Stat(clonedFile)
	if err != nil {
		t.Errorf("cloned large file not found: %v", err)
	} else if stat.Size() != int64(size) {
		t.Errorf("unexpected file size: %d", stat.Size())
	}
}

// TestGitBehaviorConcurrentAccess 测试并发访问行为
func TestGitBehaviorConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping behavior test in short mode")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found, skipping behavior test")
	}

	tmpDir := t.TempDir()
	remoteRepo := filepath.Join(tmpDir, "remote.git")

	// 创建裸仓库
	runGit(t, gitPath, tmpDir, "init", "--bare", "remote.git")

	// 创建多个克隆并同时推送
	numClients := 3
	done := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			clientDir := filepath.Join(tmpDir, fmt.Sprintf("client_%d", clientID))

			// 克隆
			runGit(t, gitPath, tmpDir, "clone", remoteRepo, fmt.Sprintf("client_%d", clientID))

			// 创建文件
			testFile := filepath.Join(clientDir, fmt.Sprintf("file_%d.txt", clientID))
			content := fmt.Sprintf("content from client %d", clientID)
			if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
				t.Errorf("failed to write file: %v", err)
				done <- false
				return
			}

			runGit(t, gitPath, clientDir, "add", fmt.Sprintf("file_%d.txt", clientID))
			runGit(t, gitPath, clientDir, "config", "user.email", fmt.Sprintf("client%d@example.com", clientID))
			runGit(t, gitPath, clientDir, "config", "user.name", fmt.Sprintf("Client %d", clientID))
			runGit(t, gitPath, clientDir, "commit", "-m", fmt.Sprintf("Commit from client %d", clientID))

			// 推送可能失败（并发冲突），这是正常的
			runGit(t, gitPath, clientDir, "push", remoteRepo, "master")

			done <- true
		}(i)
	}

	// 等待所有客户端完成
	for i := 0; i < numClients; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("timeout waiting for concurrent clients")
		}
	}
}

// TestGitBehaviorInterruptedTransfer 测试中
