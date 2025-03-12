package database

import (
	"smart-git/config"
	"smart-git/database/bolt"
	"smart-git/database/schema"
)

var (
	DB DataAccess
)

type DataAccess interface {
	SaveData(*schema.RepoData) error
	GetData(string, string) (*schema.RepoData, bool, error)
	GetAllData() ([]schema.RepoData, error)

	SaveSumData(*schema.RepoSumData) error
	GetSumData(string, string) (*schema.RepoSumData, bool, error)
	GetAllSumData() ([]schema.RepoSumData, error)
}

func SetDBInfo(cfg *config.Config) {
	DB = bolt.OpenDatabase(cfg.Database.Path)
}
