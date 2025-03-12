package bolt

//存入条目
import (
	"encoding/json"
	"fmt"
	"smart-git/database/schema"

	"github.com/WJQSERVER-STUDIO/go-utils/logger"
	"go.etcd.io/bbolt"
)

// SaveSumData 存入条目
func (s *Storage) SaveSumData(data *schema.RepoSumData) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		// 使用数据制作key
		key := data.RepoUser + "/" + data.RepoName

		// 序列化为 JSON 格式
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return err
		}

		// 创建或获取存储桶
		bucket, err := tx.CreateBucketIfNotExists([]byte(sumBucketName))
		if err != nil {
			return err
		}

		// 根据 UUID 存储数据
		return bucket.Put([]byte(key), dataBytes)
	})
}

// GetSumData 获取条目
func (s *Storage) GetSumData(repoUser string, repoName string) (*schema.RepoSumData, bool, error) {
	var repoSumData schema.RepoSumData
	key := repoUser + "/" + repoName
	found := false // 初始化 found 为 false (默认未找到)
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sumBucketName))
		if bucket == nil {
			return nil
		}

		// 获取数据
		dataBytes := bucket.Get([]byte(key))
		if dataBytes == nil {
			return nil
		}

		// 反序列化 JSON 数据
		if err := json.Unmarshal(dataBytes, &repoSumData); err != nil {
			return fmt.Errorf("JSON 反序列化失败: %w", err)
		}

		found = true
		return nil
	})

	if err != nil {
		return nil, false, fmt.Errorf("GetSumData 失败: %w", err) // 处理 View 事务执行过程中发生的错误
	}

	return &repoSumData, found, err
}

var (
	logw       = logger.Logw
	logDump    = logger.LogDump
	logDebug   = logger.LogDebug
	logInfo    = logger.LogInfo
	logWarning = logger.LogWarning
	logError   = logger.LogError
)

// 检出所有条目(debug) 日志输出
func (s *Storage) GetAllSumData() ([]schema.RepoSumData, error) {
	var records []schema.RepoSumData

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(sumBucketName))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var record schema.RepoSumData
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			records = append(records, record)
		}

		return nil
	})

	// 输出调试日志
	for _, record := range records {
		logDebug("Record: RepoUser: %s, RepoName: %s, CloneCount: %d, RequestCount: %d",
			record.RepoUser,
			record.RepoName,
			record.CloneCount,
			record.RequestCount,
		)
	}

	return records, err
}
