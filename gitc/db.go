package gitc

// 构建并记录仓库数据
import (
	"smart-git/database"
	"smart-git/database/schema"
	"time"
)

// 存入条目
func SaveRepoData(repoUrl string, repoUser string, repoName string, expireTime time.Duration) error {
	headHash, err := GetRemoteHeadHash(repoUrl)
	if err != nil {
		logError("Fail to get head hash: %v\n", err)
		return err
	}
	repoData := &schema.RepoData{
		DownloadedTime: time.Now(),
		ExpireTime:     time.Now().Add(expireTime), // 过期
		RepoURL:        repoUrl,
		RepoUser:       repoUser,
		RepoName:       repoName,
		RepoCommitHash: headHash,
	}
	err = database.DB.SaveData(repoData)
	if err != nil {
		logError("Fail to save repo data: %v\n", err)
		return err
	}
	return nil
}

// 更新条目
func UpdateRepoData(repoUrl string, repoUser string, repoName string, expireExTime time.Duration) error {
	headHash, err := GetRemoteHeadHash(repoUrl)
	if err != nil {
		logError("Fail to get head hash: %v\n", err)
		return err
	}
	repoData := &schema.RepoData{
		DownloadedTime: time.Now(),
		ExpireTime:     time.Now().Add(expireExTime), // 过期
		RepoURL:        repoUrl,
		RepoUser:       repoUser,
		RepoName:       repoName,
		RepoCommitHash: headHash,
	}
	err = database.DB.SaveData(repoData)
	if err != nil {
		logError("Fail to save repo data: %v\n", err)
		return err
	}
	return nil
}

// 检出条目
func GetRepoData(repoUser string, repoName string) (*schema.RepoData, bool, error) {
	repoData, isExist, err := database.DB.GetData(repoUser, repoName)
	if err != nil {
		logError("Fail to get repo data: %v\n", err)
		return nil, false, err
	}

	return repoData, isExist, nil
}

// 提取过期时间与hash
func GetRepoExpireInfo(repoUser string, repoName string) (time.Time, string, error) {
	repoData, isExit, err := GetRepoData(repoUser, repoName)
	if err != nil {
		logError("Fail to get repo data: %v\n", err)
		return time.Time{}, "", err
	}
	if !isExit {
		logError("Repo data not exist: %v\n", err)
		return time.Time{}, "", err
	}
	return repoData.ExpireTime, repoData.RepoCommitHash, nil
}

// 存入条目
func SaveSumData(sumData *schema.RepoSumData, repoUser string, repoName string) error {
	err := database.DB.SaveSumData(sumData)
	if err != nil {
		logError("Fail to save repo sum data: %v\n", err)
		return err
	}
	return nil
}

// 检出条目
func GetSumData(repoUser string, repoName string) (*schema.RepoSumData, bool, error) {
	repoSumData, isExist, err := database.DB.GetSumData(repoUser, repoName)
	if err != nil {
		logError("Fail to get repo sum data: %v\n", err)
		return nil, false, err
	}
	return repoSumData, isExist, nil
}

// CloneCount +1
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
	}
	return nil
}
