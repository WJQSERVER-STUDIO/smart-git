package gitc

import (
	"errors"
	"fmt"
	"os"
	"time"

	"smart-git/config"

	"github.com/WJQSERVER-STUDIO/logger"
	"github.com/go-git/go-git/v5"
	gconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

var (
	logw       = logger.Logw
	logDump    = logger.LogDump
	logDebug   = logger.LogDebug
	logInfo    = logger.LogInfo
	logWarning = logger.LogWarning
	logError   = logger.LogError
)

func CloneRepo(dir string, userName string, repoName string, repoUrl string, cfg *config.Config) error {
	repoPath := dir

	// 预检测文件夹问题: 检查目录是否已经存在
	_, err := os.Stat(repoPath)
	if err == nil { // 目录存在
		// 检查它是否是一个 git 仓库
		_, err = git.PlainOpen(repoPath)
		if err != nil {
			// 目录存在，但不是一个有效的 git 仓库
			logError("目录 '%s' 存在，但不是一个有效的 git 仓库。移除并重新克隆。\n", repoPath)
			err = os.RemoveAll(repoPath)
			if err != nil {
				logError("移除无效仓库目录失败: %v\n", err)
				return err
			}
			// 继续克隆
		} else {
			// 目录存在，并且是一个 git 仓库
			// 判断是否过期
			var (
				expireTime time.Time
				headHash   string
				err        error
			)

			expireTime, headHash, err = GetRepoExpireInfo(userName, repoName)
			if err != nil {
				logError("获取仓库过期时间失败: %v\n", err)
				return err
			}
			if expireTime.Before(time.Now()) {
				// 过期

				// 检测hash 若一致则证明无需重新拉取
				remoteHeadHash, err := GetRemoteHeadHash(repoUrl)
				if err != nil {
					logError("获取远程仓库 HEAD 失败: %v\n", err)
					return err
				}
				if remoteHeadHash == headHash {
					logInfo("仓库 '%s' 已经存在, 超过过期时间, 但 hash 是最新的。\n", repoPath)
					// 写入 db
					err = UpdateRepoData(repoUrl, userName, repoName, cfg.Cache.ExpireEx)
					if err != nil {
						logError("保存仓库数据失败: %v\n", err)
						return err
					}
					return nil // 仓库未过期，直接使用
				}
				logInfo("仓库 '%s' 已过期。移除并重新克隆。\n", repoPath)
				err = os.RemoveAll(repoPath)
				if err != nil {
					logError("移除过期仓库失败: %v\n", err)
					return err
				}
				// 继续克隆
			} else {
				logInfo("仓库 '%s' 已经存在且是最新的。\n", repoPath)
				return nil // 仓库未过期，直接使用
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		// os.Stat 错误，不是目录不存在
		logError("检查目录 '%s' 时出错: %v\n", repoPath, err)
		return err
	}
	// 如果目录不存在，或者因为过期或无效仓库而被移除，则克隆它

	_, err = git.PlainClone(repoPath, true, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
		Mirror:   true,
	})
	if err != nil {
		logError("克隆仓库 '%s' 失败: %v\n", repoUrl, err)
		return err

	} else {
		err := AddCloneCount(userName, repoName)
		if err != nil {
			logError("增加克隆次数失败: %v\n", err)
			return err
		}
	}

	// 写入 db
	err = SaveRepoData(repoUrl, userName, repoName, cfg.Cache.Expire)
	if err != nil {
		logError("保存仓库数据失败: %v\n", err)
		return err
	}

	/*
	   // 压缩
	   go func() {
	       err := CompressRepo(repoPath)
	       if err != nil {
	           logError("压缩失败: %v\n", err)
	       } else {
	           logInfo("压缩成功: %s.lz4\n", repoPath)
	       }
	   }()
	*/

	return nil
}

// GetRemoteHeadHash 函数用于获取远程仓库 HEAD 指向的 commit hash
func GetRemoteHeadHash(repoURL string) (string, error) {
	// 创建一个远程仓库对象，使用内存存储
	rem := git.NewRemote(memory.NewStorage(), &gconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	// 获取远程仓库的 HEAD 引用
	ref, err := rem.List(&git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("获取远程仓库引用列表失败: %w", err)
	}

	var mainRef string
	// 遍历引用列表，查找 HEAD 引用
	for _, reference := range ref {
		if reference.Name() == plumbing.HEAD {
			mainRef = reference.Target().String()
			break
		}
	}

	// 查找mainRef对应的hash
	for _, reference := range ref {
		if reference.Name().String() == mainRef {
			logDebug("Main ref: %s, hash: %s, target: %s", reference.Name(), reference.Hash(), reference.Target())
			return reference.Hash().String(), nil
		}
	}

	// 如果没有找到 HEAD 引用，返回错误
	return "", fmt.Errorf("未找到远程仓库 HEAD 引用")
}

/*
// CompressRepo 将指定的仓库压缩成 LZ4 格式的压缩包
func CompressRepo(repoPath string) error {
	lz4File, err := os.Create(repoPath + ".lz4")
	if err != nil {
		return fmt.Errorf("failed to create LZ4 file: %w", err)
	}
	defer lz4File.Close()

	// 创建 LZ4 编码器
	lz4Writer := lz4.NewWriter(lz4File)
	defer lz4Writer.Close()

	// 创建 tar.Writer
	tarBuffer := new(bytes.Buffer)
	tarWriter := tar.NewWriter(tarBuffer)

	// 遍历仓库目录并打包
	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建 tar 文件头
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name, err = filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		// 写入 tar 文件头
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，写入文件内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk through repo directory: %w", err)
	}

	// 关闭 tar.Writer
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// 将 tar 数据写入 LZ4 压缩包
	if _, err := lz4Writer.Write(tarBuffer.Bytes()); err != nil {
		return fmt.Errorf("failed to write to LZ4 file: %w", err)
	}

	return nil
}
*/
