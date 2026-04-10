package main

import (
	"time"

	"smart-git/database/schema"

	wanfcodec "github.com/WJQSERVER/wanf"
	"github.com/infinite-iroha/touka"
)

type APIRepoRecord struct {
	Owner       string `wanf:"owner" json:"owner"`
	Name        string `wanf:"name" json:"name"`
	UpstreamURL string `wanf:"upstream_url" json:"upstream_url"`
	LocalPath   string `wanf:"local_path" json:"local_path"`
	HeadOID     string `wanf:"head_oid,omitempty" json:"head_oid,omitempty"`
	Status      string `wanf:"status" json:"status"`
	CreatedAt   string `wanf:"created_at" json:"created_at"`
	UpdatedAt   string `wanf:"updated_at" json:"updated_at"`
	ExpiresAt   string `wanf:"expires_at" json:"expires_at"`
}

type APIRepoStats struct {
	Owner        string `wanf:"owner" json:"owner"`
	Name         string `wanf:"name" json:"name"`
	CloneCount   int    `wanf:"clone_count" json:"clone_count"`
	RequestCount int    `wanf:"request_count" json:"request_count"`
}

type APIRepoRecordList struct {
	Items []APIRepoRecord `wanf:"items" json:"items"`
}

type APIRepoStatsList struct {
	Items []APIRepoStats `wanf:"items" json:"items"`
}

type APIHealthResponse struct {
	Status       string `wanf:"status" json:"status"`
	RepoDir      string `wanf:"repo_dir" json:"repo_dir"`
	DatabasePath string `wanf:"database_path" json:"database_path"`
	GithubBase   string `wanf:"github_base" json:"github_base"`
}

type APISyncResponse struct {
	Owner       string `wanf:"owner" json:"owner"`
	Name        string `wanf:"name" json:"name"`
	UpstreamURL string `wanf:"upstream_url" json:"upstream_url"`
	LocalPath   string `wanf:"local_path" json:"local_path"`
	HeadOID     string `wanf:"head_oid,omitempty" json:"head_oid,omitempty"`
	Status      string `wanf:"status" json:"status"`
	FreshClone  bool   `wanf:"fresh_clone" json:"fresh_clone"`
	Refreshed   bool   `wanf:"refreshed" json:"refreshed"`
}

type APIErrorResponse struct {
	Error string `wanf:"error" json:"error"`
}

func RenderWANF(c *touka.Context, code int, obj any) {
	c.SetHeader("Content-Type", "application/vnd.wjqserver.wanf; charset=utf-8")
	c.SetHeader("X-Content-Type-Options", "nosniff")
	c.Writer.WriteHeader(code)

	encoder := wanfcodec.NewNeoEncoder(c.Writer)
	if err := encoder.Encode(obj); err != nil {
		encoder.Close()
		logError("failed to encode WANF response: %v", err)
		return
	}
	encoder.Close()
}

func RenderWANFError(c *touka.Context, code int, message string) {
	RenderWANF(c, code, &APIErrorResponse{Error: message})
}

func NewAPIRepoRecord(record schema.RepoData) APIRepoRecord {
	return APIRepoRecord{
		Owner:       record.RepoUser,
		Name:        record.RepoName,
		UpstreamURL: record.RepoURL,
		LocalPath:   record.LocalPath,
		HeadOID:     record.RepoCommitHash,
		Status:      record.Status,
		CreatedAt:   formatTime(record.DownloadedTime),
		UpdatedAt:   formatTime(record.UpdatedTime),
		ExpiresAt:   formatTime(record.ExpireTime),
	}
}

func NewAPIRepoStats(record schema.RepoSumData) APIRepoStats {
	return APIRepoStats{
		Owner:        record.RepoUser,
		Name:         record.RepoName,
		CloneCount:   record.CloneCount,
		RequestCount: record.RequestCount,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
