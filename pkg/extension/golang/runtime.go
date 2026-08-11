package golang

import (
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// ExtensionRuntime adapts the Golang (Scriggo) extension package to the shared
// endpoint.Runtime interface. It satisfies the interface structurally (no
// import of the endpoint package is required).
//
// The Golang backend is v2-only: extensions are always compiled against the
// v2 API, so there is no v1 legacy path to handle here. It is named
// ExtensionRuntime to avoid colliding with the VM Runtime type in vm.go.
type ExtensionRuntime struct{}

func (ExtensionRuntime) GetExtensionMeta(pkg string) (*extension.Extension, error) {
	return GetExtensionMeta(pkg)
}

func (ExtensionRuntime) Latest(pkg string, page int) ([]*proto.ExtensionListItem, error) {
	return Latest(pkg, page)
}

func (ExtensionRuntime) Search(pkg string, page int, kw string, filter *proto.FilterSelection) ([]*proto.ExtensionListItem, error) {
	return Search(pkg, page, kw, filter)
}

func (ExtensionRuntime) Watch(pkg string, url string) (any, *extension.Extension, error) {
	return Watch(pkg, url)
}

func (ExtensionRuntime) Detail(pkg string, url string) (*proto.ExtensionDetail, error) {
	return Detail(pkg, url)
}

func (ExtensionRuntime) Mirror(pkg string, url string) (any, error) {
	return Mirror(pkg, url)
}

func (ExtensionRuntime) CreateFilter(pkg string, filter *proto.FilterSelection) (map[string]*proto.ExtensionFilter, error) {
	return CreateFilter(pkg, filter)
}
