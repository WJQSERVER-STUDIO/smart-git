package gitc

import (
	"smart-git/database"
	"smart-git/database/schema"
	"time"
)

const (
	RepoStatusPending = "pending"
	RepoStatusSynced  = "synced"
)

func SaveRepoData(data *schema.RepoData) error {
	err := database.DB.SaveData(data)
	if err != nil {
		logError("Fail to save repo data: %v\n", err)
		return err
	}
	return nil
}

func SavePendingRepoData(repoURL string, repoUser string, repoName string, localPath string) error {
	now := time.Now()
	repoData := &schema.RepoData{
		DownloadedTime: now,
		UpdatedTime:    now,
		ExpireTime:     now,
		RepoURL:        repoURL,
		LocalPath:      localPath,
		RepoUser:       repoUser,
		RepoName:       repoName,
		RepoCommitHash: "",
		Status:         RepoStatusPending,
	}
	return SaveRepoData(repoData)
}

func SaveSyncedRepoData(repoURL string, repoUser string, repoName string, localPath string, headHash string, expireTime time.Duration) error {
	now := time.Now()
	downloadedTime := now
	if current, exists, err := GetRepoData(repoUser, repoName); err == nil && exists && current.DownloadedTime.After(time.Time{}) {
		downloadedTime = current.DownloadedTime
	}
	repoData := &schema.RepoData{
		DownloadedTime: downloadedTime,
		UpdatedTime:    now,
		ExpireTime:     now.Add(expireTime),
		RepoURL:        repoURL,
		LocalPath:      localPath,
		RepoUser:       repoUser,
		RepoName:       repoName,
		RepoCommitHash: headHash,
		Status:         RepoStatusSynced,
	}
	return SaveRepoData(repoData)
}

func ExtendRepoExpire(repoData *schema.RepoData, expireExTime time.Duration) error {
	now := time.Now()
	repoData.UpdatedTime = now
	repoData.ExpireTime = now.Add(expireExTime)
	repoData.Status = RepoStatusSynced
	return SaveRepoData(repoData)
}

func GetRepoData(repoUser string, repoName string) (*schema.RepoData, bool, error) {
	repoData, isExist, err := database.DB.GetData(repoUser, repoName)
	if err != nil {
		logError("Fail to get repo data: %v\n", err)
		return nil, false, err
	}

	return repoData, isExist, nil
}

func GetAllRepoData() ([]schema.RepoData, error) {
	records, err := database.DB.GetAllData()
	if err != nil {
		logError("Fail to get all repo data: %v\n", err)
		return nil, err
	}
	return records, nil
}

func DeleteRepoData(repoUser string, repoName string) error {
	err := database.DB.DeleteData(repoUser, repoName)
	if err != nil {
		logError("Fail to delete repo data: %v\n", err)
		return err
	}
	return nil
}

func SaveSumData(sumData *schema.RepoSumData, repoUser string, repoName string) error {
	err := database.DB.SaveSumData(sumData)
	if err != nil {
		logError("Fail to save repo sum data: %v\n", err)
		return err
	}
	return nil
}

func GetSumData(repoUser string, repoName string) (*schema.RepoSumData, bool, error) {
	repoSumData, isExist, err := database.DB.GetSumData(repoUser, repoName)
	if err != nil {
		logError("Fail to get repo sum data: %v\n", err)
		return nil, false, err
	}
	return repoSumData, isExist, nil
}

func AddCloneCount(repoUser string, repoName string) error {
	repoSumData, isExist, err := GetSumData(repoUser, repoName)
	if err != nil {
		logError("Fail to get repo sum data: %v\n", err)
		return err
	}
	if isExist == false {
		repoSumData = &schema.RepoSumData{
			RepoUser:     repoUser,
			RepoName:     repoName,
			CloneCount:   0,
			RequestCount: 0,
		}
	}
	repoSumData.CloneCount++
	err = database.DB.SaveSumData(repoSumData)
	if err != nil {
		logError("Fail to save repo sum data: %v\n", err)
		return err
	}
	return nil
}
