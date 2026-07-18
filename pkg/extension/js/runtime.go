package js

import (
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// Runtime adapts the JS extension package to the shared endpoint.Runtime
// interface. It satisfies the interface structurally (no import of the
// endpoint package is required).
//
// The JS backend supports both the legacy v1 API and the v2 API; the v1/v2
// branching is handled internally by Watch/Mirror via handleMediaType and by
// the eval strings configured per ApiVersion.
type Runtime struct{}

func (Runtime) GetExtensionMeta(pkg string) (*extension.Extension, error) {
	return GetExtensionMeta(pkg)
}

func (Runtime) Latest(pkg string, page int) ([]*proto.ExtensionListItem, error) {
	return Latest[proto.ExtensionListItem](pkg, page)
}

func (Runtime) Search(pkg string, page int, kw string, filter string) ([]*proto.ExtensionListItem, error) {
	return Search(pkg, page, kw, filter)
}

func (Runtime) Watch(pkg string, url string) (any, *extension.Extension, error) {
	return Watch(pkg, url)
}

func (Runtime) Detail(pkg string, url string) (*proto.ExtensionDetail, error) {
	return Detail(pkg, url)
}

func (Runtime) Mirror(pkg string, url string) (any, error) {
	return Mirror(pkg, url)
}

func (Runtime) CreateFilter(pkg string, filter string) (map[string]*proto.ExtensionFilter, error) {
	return CreateFilter(pkg, filter)
}
