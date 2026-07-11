package extension

type Extension struct {
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

	// FileLang is the runtime language detected from the source file's
	// extension (".js" -> LanguageJS, ".go" -> LanguageGolang). It is set by
	// ParseExtensionMetadata and lets each runtime route an extension to the
	// correct backend without re-deriving the language from the (metadata)
	// Name field. The metadata @lang value is unrelated and lives in Lang.
	FileLang Language `json:"-"`
}
