package gitc

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"smart-git/config"
	"smart-git/database/schema"

	"github.com/WJQSERVER-STUDIO/logger"
	"github.com/go-git/go-git/v6"
	gconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

var (
	logw       = logger.Logw
	logDump    = logger.LogDump
	logDebug   = logger.LogDebug
	logInfo    = logger.LogInfo
	logWarning = logger.LogWarning
	logError   = logger.LogError

	repoLocksMu sync.Mutex
	repoLocks   = map[string]*repoLockEntry{}
)

type repoLockEntry struct {
	mu   sync.Mutex
	refs int
}

func EnsureRepoReady(basedir string, userName string, repoName string, repoURL string, cfg *config.Config) error {
	lockKey := userName + "/" + repoName
	lock := acquireRepoLock(lockKey)
	defer releaseRepoLock(lockKey, lock)

	return syncRepoLocked(basedir, userName, repoName, repoURL, cfg)
}

func RecoverPendingRepos(cfg *config.Config) error {
	records, err := GetAllRepoData()
	if err != nil {
		return err
	}

	for _, record := range records {
		if record.Status != RepoStatusPending {
			continue
		}

		if repoIsUsable(record.LocalPath) {
			headHash, err := LocalHeadHash(record.LocalPath)
			if err != nil {
				logError("recover pending repo head failed: %v, repo: %s/%s\n", err, record.RepoUser, record.RepoName)
				if err := removeRepoArtifacts(record); err != nil {
					return err
				}
				continue
			}

			if err := SaveSyncedRepoData(record.RepoURL, record.RepoUser, record.RepoName, record.LocalPath, headHash, cfg.Cache.ExpireEx); err != nil {
				return err
			}
			continue
		}

		if err := removeRepoArtifacts(record); err != nil {
			return err
		}
	}

	return nil
}

func syncRepoLocked(basedir string, userName string, repoName string, repoURL string, cfg *config.Config) error {
	localPath := filepath.Join(basedir, userName, repoName)
	repoData, exists, err := GetRepoData(userName, repoName)
	if err != nil {
		return err
	}

	if exists && repoData.LocalPath == "" {
		repoData.LocalPath = localPath
	}

	if exists && repoData.Status == RepoStatusPending {
		if repoIsUsable(localPath) {
			return finalizeSyncedRepo(localPath, repoURL, userName, repoName, cfg.Cache.ExpireEx)
		}
		if err := removeRepoArtifacts(*repoData); err != nil {
			return err
		}
		repoData = nil
		exists = false
	}

	if exists && repoIsUsable(localPath) {
		if repoData.Status != RepoStatusSynced {
			return finalizeSyncedRepo(localPath, repoURL, userName, repoName, cfg.Cache.ExpireEx)
		}
		if repoData.ExpireTime.After(time.Now()) {
			logInfo("仓库 '%s' 已经存在且在有效期内。\n", localPath)
			return nil
		}
		return refreshExistingRepo(localPath, repoURL, userName, repoName, cfg, repoData)
	}

	if !exists && repoIsUsable(localPath) {
		logWarning("仓库 '%s' 存在但缺少元数据，自动修复记录。\n", localPath)
		return finalizeSyncedRepo(localPath, repoURL, userName, repoName, cfg.Cache.ExpireEx)
	}

	if stat, statErr := os.Stat(localPath); statErr == nil && stat.IsDir() {
		logWarning("仓库目录 '%s' 存在但不可用，准备重建。\n", localPath)
		if err := os.RemoveAll(localPath); err != nil {
			return err
		}
	}

	if exists {
		if err := DeleteRepoData(userName, repoName); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	if err := SavePendingRepoData(repoURL, userName, repoName, localPath); err != nil {
		return err
	}

	_, err = git.PlainClone(localPath, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
		Mirror:   true,
		Bare:     true,
	})
	if err != nil {
		_ = DeleteRepoData(userName, repoName)
		_ = os.RemoveAll(localPath)
		logError("克隆仓库 '%s' 失败: %v\n", repoURL, err)
		return err
	}

	if err := AddCloneCount(userName, repoName); err != nil {
		return err
	}

	return finalizeSyncedRepo(localPath, repoURL, userName, repoName, cfg.Cache.Expire)
}

func refreshExistingRepo(localPath string, repoURL string, userName string, repoName string, cfg *config.Config, repoData *schema.RepoData) error {
	if err := SavePendingRepoData(repoURL, userName, repoName, localPath); err != nil {
		return err
	}

	repo, err := git.PlainOpen(localPath)
	if err != nil {
		_ = DeleteRepoData(repoData.RepoUser, repoData.RepoName)
		return err
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		_ = DeleteRepoData(repoData.RepoUser, repoData.RepoName)
		return err
	}

	fetchErr := remote.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []gconfig.RefSpec{
			gconfig.RefSpec("+refs/*:refs/*"),
		},
		Prune:    true,
		Progress: os.Stdout,
		Tags:     plumbing.AllTags,
		Force:    true,
	})
	if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		_ = restoreSyncedRepoData(repoData, cfg.Cache.ExpireEx)
		logError("fetch 仓库 '%s' 失败: %v\n", repoURL, fetchErr)
		return fetchErr
	}

	localHeadHash, err := LocalHeadHash(localPath)
	if err != nil {
		_ = restoreSyncedRepoData(repoData, cfg.Cache.ExpireEx)
		return err
	}

	if errors.Is(fetchErr, git.NoErrAlreadyUpToDate) || localHeadHash == repoData.RepoCommitHash {
		logInfo("仓库 '%s' 经过 fetch 检查后仍是最新。\n", localPath)
		return ExtendRepoExpire(repoData, cfg.Cache.ExpireEx)
	}

	return finalizeSyncedRepo(localPath, repoURL, userName, repoName, cfg.Cache.Expire)
}

func finalizeSyncedRepo(localPath string, repoURL string, userName string, repoName string, expire time.Duration) error {
	headHash, err := LocalHeadHash(localPath)
	if err != nil {
		return err
	}
	return SaveSyncedRepoData(repoURL, userName, repoName, localPath, headHash, expire)
}

func LocalHeadHash(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func repoIsUsable(repoPath string) bool {
	if repoPath == "" {
		return false
	}
	if _, err := os.Stat(repoPath); err != nil {
		return false
	}
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false
	}
	return repo.Storer != nil
}

func removeRepoArtifacts(repoData schema.RepoData) error {
	if repoData.LocalPath != "" {
		if err := os.RemoveAll(repoData.LocalPath); err != nil {
			return err
		}
	}
	return DeleteRepoData(repoData.RepoUser, repoData.RepoName)
}

func restoreSyncedRepoData(repoData *schema.RepoData, expire time.Duration) error {
	if repoData == nil {
		return nil
	}
	return SaveSyncedRepoData(repoData.RepoURL, repoData.RepoUser, repoData.RepoName, repoData.LocalPath, repoData.RepoCommitHash, expire)
}

func acquireRepoLock(key string) *repoLockEntry {
	repoLocksMu.Lock()
	entry, ok := repoLocks[key]
	if !ok {
		entry = &repoLockEntry{}
		repoLocks[key] = entry
	}
	entry.refs++
	repoLocksMu.Unlock()

	entry.mu.Lock()
	return entry
}

func releaseRepoLock(key string, entry *repoLockEntry) {
	entry.mu.Unlock()

	repoLocksMu.Lock()
	entry.refs--
	if entry.refs == 0 {
		delete(repoLocks, key)
	}
	repoLocksMu.Unlock()
}
