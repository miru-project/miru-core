package jsExtension

type Ext struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	License     string   `json:"license"`
	Lang        string   `json:"lang"`
	Icon        string   `json:"icon"`
	Pkg         string   `json:"package"`
	Website     string   `json:"webSite"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	ApiVersion  string   `json:"apiVersion"`
	Error       string   `json:"error,omitempty"`
	Context     *string
	WatchType   string `json:"type"`
}
