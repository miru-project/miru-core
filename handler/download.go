package handler

import (
	"strconv"

	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/result"
)

func DownloadBangumi(filePath string, url string, header map[string]string, isHLS bool) (*result.Result, error) {

	res, err := download.DownloadBangumi(filePath, url, header, isHLS)
	if err != nil {
		return result.NewErrorResult("Failed to download bangumi", 500), err
	}

	return result.NewSuccessResult(res), nil
}

func DownloadStatus() *result.Result {
	res := download.DownloadStatus()
	return result.NewSuccessResult(res)
}

func CancelTask(taskId string) (*result.Result, error) {
	id, err := strconv.Atoi(taskId)
	if err != nil {
		return result.NewErrorResult("Invalid task ID", 400), err
	}

	err = download.CancelTask(id)
	if err != nil {
		return result.NewErrorResult("Failed to cancel task", 500), err
	}

	return result.NewSuccessResult("ok"), nil
}

func ResumeTask(taskId string) (*result.Result, error) {
	id, err := strconv.Atoi(taskId)
	if err != nil {
		return result.NewErrorResult("Invalid task ID", 400), err
	}

	err = download.ResumeTask(id)
	if err != nil {
		return result.NewErrorResult("Failed to resume task", 500), err
	}

	return result.NewSuccessResult("ok"), nil
}

func PauseTask(taskId string) (*result.Result, error) {
	id, err := strconv.Atoi(taskId)
	if err != nil {
		return result.NewErrorResult("Invalid task ID", 400), err
	}

	err = download.PauseTask(id)
	if err != nil {
		return result.NewErrorResult("Failed to pause task", 500), err
	}

	return result.NewSuccessResult("ok"), nil
}
