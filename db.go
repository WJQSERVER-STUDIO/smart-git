package main

import (
	"smart-git/database"
	"smart-git/database/schema"
)

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
		return err
	}
	return nil
}

// RequestCount +1
func AddRequestCount(repoUser string, repoName string) error {
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
	repoSumData.RequestCount++
	err = database.DB.SaveSumData(repoSumData)
	if err != nil {
		logError("Fail to save repo sum data: %v\n", err)
		return err
	}
	return nil
}
