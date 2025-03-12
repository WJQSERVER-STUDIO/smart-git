package bolt

import (
	"encoding/json"
	"fmt"
	"smart-git/database/schema"

	"go.etcd.io/bbolt"
)

/*
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
*/

// 存入条目
func (s *Storage) SaveData(data *schema.RepoData) error {
	return s.db.Update(func(tx *bbolt.Tx) error {

		// 使用数据制作key
		key := data.RepoUser + "/" + data.RepoName

		// 序列化为 JSON 格式
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return err
		}

		// 创建或获取存储桶
		bucket, err := tx.CreateBucketIfNotExists([]byte(dataBucketName))
		if err != nil {
			return err
		}

		// 根据 UUID 存储数据
		return bucket.Put([]byte(key), dataBytes)
	})
}

func (s *Storage) GetData(repoUser string, repoName string) (*schema.RepoData, bool, error) {
	var repoData schema.RepoData
	key := repoUser + "/" + repoName
	found := false                              // 初始化 found 为 false (默认未找到)
	err := s.db.View(func(tx *bbolt.Tx) error { //  <--  单次 View 事务 !!!
		bucket := tx.Bucket([]byte(dataBucketName))
		if bucket == nil {
			return nil // Bucket 不存在仍然返回 nil error
		}

		// 获取数据
		dataBytes := bucket.Get([]byte(key))
		if dataBytes == nil {
			return nil // Key 不存在时，匿名函数返回 nil error, 此时 found 仍然为 false (默认值)
		}

		// 反序列化 JSON 数据 (只有在 dataBytes != nil 时才执行)
		if err := json.Unmarshal(dataBytes, &repoData); err != nil {
			return fmt.Errorf("JSON 反序列化失败: %w", err) // JSON 反序列化错误
		}

		found = true //  <--  在成功获取到 dataBytes 后，将 found 设置为 true
		return nil
	})

	if err != nil {
		return nil, false, fmt.Errorf("GetData 失败: %w", err) // 处理 View 事务执行过程中发生的错误
	}

	return &repoData, found, nil // 返回 RepoData, found bool 值, 和 nil error (或者包装后的错误)
}

func (s *Storage) GetAllData() ([]schema.RepoData, error) {
	var records []schema.RepoData

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(dataBucketName))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var record schema.RepoData
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			records = append(records, record)
		}

		return nil
	})

	// 输出调试日志
	for _, record := range records {
		logDebug("Record: RepoUser: %s, RepoName: %s, RepoURL: %s, RepoCommitHash: %s, DownloadedTime: %s, ExpireTime: %s",
			record.RepoUser,
			record.RepoName,
			record.RepoURL,
			record.RepoCommitHash,
			record.DownloadedTime.Format("2006-01-02 15:04:05"),
			record.ExpireTime.Format("2006-01-02 15:04:05"),
		)
	}

	return records, err
}
