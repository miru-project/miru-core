package extension

import "fmt"

// WatchType is the constrained set of content kinds an extension can watch. It
// is a string-backed enum limited to exactly four values; the parser rejects
// any @type that is not one of these.
type WatchType string

const (
	// WatchTypeBangumi is video streaming (anime) content.
	WatchTypeBangumi WatchType = "bangumi"
	// WatchTypeManga is comic / image-page content.
	WatchTypeManga WatchType = "manga"
	// WatchTypeFikushon is novel / text content.
	WatchTypeFikushon WatchType = "fikushon"
	// WatchTypeAll is the combined type carrying manga + fikushon + bangumi.
	WatchTypeAll WatchType = "all"
)

// ParseWatchType validates a raw @type string against the allowed WatchType
// values. It returns an error for anything other than the four known kinds so
// that non-conforming extensions fail fast at load time.
func ParseWatchType(s string) (WatchType, error) {
	switch WatchType(s) {
	case WatchTypeBangumi, WatchTypeManga, WatchTypeFikushon, WatchTypeAll:
		return WatchType(s), nil
	default:
		return "", fmt.Errorf("unsupported @type %q: must be one of bangumi, manga, fikushon, all", s)
	}
}

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
	WatchType   WatchType `json:"type"`

	// FileLang is the runtime language detected from the source file's
	// extension (".js" -> LanguageJS, ".go" -> LanguageGolang). It is set by
	// ParseExtensionMetadata and lets each runtime route an extension to the
	// correct backend without re-deriving the language from the (metadata)
	// Name field. The metadata @lang value is unrelated and lives in Lang.
	FileLang Language `json:"-"`
}
