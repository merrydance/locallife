package ocr

import (
	"fmt"
)

// Router resolves OCR provider routes for document types.
type Router interface {
	Route(documentType DocumentType) (Route, error)
}

// StaticRouter resolves document types from a fixed table.
type StaticRouter struct {
	routes map[DocumentType]Route
}

// NewStaticRouter creates a router from explicit route bindings.
func NewStaticRouter(routes map[DocumentType]Route) (*StaticRouter, error) {
	if len(routes) == 0 {
		return nil, fmt.Errorf("ocr router requires at least one provider")
	}
	cloned := make(map[DocumentType]Route, len(routes))
	for documentType, route := range routes {
		if route.Provider == nil {
			return nil, fmt.Errorf("ocr route provider is required for document type: %s", documentType)
		}
		cloned[documentType] = route
	}
	return &StaticRouter{routes: cloned}, nil
}

// NewAliyunPrimaryRouter creates the default route table with Aliyun as primary provider.
func NewAliyunPrimaryRouter(aliyun Provider, wechat Provider) (*StaticRouter, error) {
	routes := make(map[DocumentType]Route)
	if aliyun != nil {
		routes[DocumentTypeBusinessLicense] = Route{Provider: aliyun, Capability: CapabilityAliyunBusinessLicense}
		routes[DocumentTypeIDCard] = Route{Provider: aliyun, Capability: CapabilityAliyunIDCard}
		routes[DocumentTypeFoodPermit] = Route{Provider: aliyun, Capability: CapabilityAliyunFoodPermit}
		routes[DocumentTypeHealthCert] = Route{Provider: aliyun, Capability: CapabilityAliyunHealthCert}
	}
	if wechat != nil {
		_ = wechat
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("ocr router requires at least one provider")
	}
	return NewStaticRouter(routes)
}

// Route resolves the configured provider route for a document type.
func (r *StaticRouter) Route(documentType DocumentType) (Route, error) {
	route, ok := r.routes[documentType]
	if !ok {
		return Route{}, fmt.Errorf("no OCR route for document type: %s", documentType)
	}
	return route, nil
}
