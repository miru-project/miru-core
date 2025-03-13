package extension

type Ext struct {
	name        string
	version     string
	author      string
	license     string
	lang        string
	icon        string
	pkg         string
	website     string
	description string
	tags        []string
	api         string
	context     *string
}

type Latest struct {
	Cover  string `json:"cover"`
	Title  string `json:"title"`
	Update string `json:"update"`
	URL    string `json:"url"`
}
type Latests []Latest
