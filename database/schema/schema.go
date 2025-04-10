package schema

import (
	"time"
)

type RepoData struct {
	// 下载时间
	DownloadedTime time.Time
	// 过期时间
	ExpireTime time.Time
	// 仓库地址
	RepoURL string
	// 仓库所有者
	RepoUser string
	// 仓库名称
	RepoName string
	// clone的Commit hash
	RepoCommitHash string
}

type RepoSumData struct {
	// 仓库所有者
	RepoUser string
	// 仓库名称
	RepoName string
	// Clone计数
	CloneCount int
	// 请求计数
	RequestCount int
}
