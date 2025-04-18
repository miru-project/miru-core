package download

import "sync"

type Progrss struct {
	Progrss int       `json:"progress"`
	Names   *[]string `json:"names"`
	Total   int       `json:"total"`
	Status  string    `json:"status"`
}

var Tasks = sync.Map{}
var Status = map[int]*Progrss{}

func DownloadStatus() map[int]*Progrss {
	// Get the status of all tasks
	return Status
}
