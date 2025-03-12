package bolt

import (
	"go.etcd.io/bbolt"
)

const (
	// 数据存储的桶名称
	dataBucketName = `smart-git`
	sumBucketName  = `smart-git-sum`
)

type Storage struct {
	db *bbolt.DB
}

// OpenDatabase 打开一个 BoltDB 数据库
func OpenDatabase(dbFilePath string) *Storage {
	db, err := bbolt.Open(dbFilePath, 0666, nil)
	if err != nil {
		logError("Failed to open BoltDB file: %s", err)
		panic(err) // 直接终止程序，确保问题被及时发现
	}
	return &Storage{db: db}
}
