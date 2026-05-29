package backend

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Capabilities struct {
	BarcodeFormats []BarcodeFormatInfo `json:"barcodeFormats" doc:"Barcode/QR formats supported by this server build"`
}

var GetCapabilitiesOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/capabilities",
	DefaultStatus: http.StatusOK,
}

func GetCapabilities(_ context.Context, _ *struct{}) (output *struct{ Body Capabilities }, err error) {
	output = &struct{ Body Capabilities }{Body: Capabilities{
		BarcodeFormats: KnownBarcodeFormats,
	}}
	return
}
