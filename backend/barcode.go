package backend

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/boxes-ltd/imaging"
	"github.com/danielgtaylor/huma/v2"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/aztec"
	"github.com/makiuchi-d/gozxing/datamatrix"
	"github.com/makiuchi-d/gozxing/multi/qrcode"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/makiuchi-d/gozxing/oned/rss"
)

type BarcodeFormatInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// KnownBarcodeFormats lists all supported scan formats with display labels.
var KnownBarcodeFormats = []BarcodeFormatInfo{
	{Value: "QR_CODE", Label: "QR Code"},
	{Value: "DATA_MATRIX", Label: "Data Matrix"},
	{Value: "AZTEC", Label: "Aztec"},
	{Value: "CODE_128", Label: "Code 128"},
	{Value: "CODE_39", Label: "Code 39"},
	{Value: "CODE_93", Label: "Code 93"},
	{Value: "CODABAR", Label: "Codabar"},
	{Value: "EAN_13", Label: "EAN-13"},
	{Value: "EAN_8", Label: "EAN-8"},
	{Value: "UPC_A", Label: "UPC-A"},
	{Value: "UPC_E", Label: "UPC-E"},
	{Value: "ITF", Label: "ITF"},
	{Value: "RSS_14", Label: "RSS-14"},
}

type formatReader struct {
	format gozxing.BarcodeFormat
	reader gozxing.Reader
}

func readerForFormat(name string) (formatReader, bool) {
	switch name {
	case "QR_CODE":
		return formatReader{gozxing.BarcodeFormat_QR_CODE, qrcode.NewQRCodeMultiReader().(gozxing.Reader)}, true
	case "DATA_MATRIX":
		return formatReader{gozxing.BarcodeFormat_DATA_MATRIX, datamatrix.NewDataMatrixReader()}, true
	case "AZTEC":
		return formatReader{gozxing.BarcodeFormat_AZTEC, aztec.NewAztecReader()}, true
	case "CODE_128":
		return formatReader{gozxing.BarcodeFormat_CODE_128, oned.NewCode128Reader()}, true
	case "CODE_39":
		return formatReader{gozxing.BarcodeFormat_CODE_39, oned.NewCode39Reader()}, true
	case "CODE_93":
		return formatReader{gozxing.BarcodeFormat_CODE_93, oned.NewCode93Reader()}, true
	case "CODABAR":
		return formatReader{gozxing.BarcodeFormat_CODABAR, oned.NewCodaBarReader()}, true
	case "EAN_13":
		return formatReader{gozxing.BarcodeFormat_EAN_13, oned.NewEAN13Reader()}, true
	case "EAN_8":
		return formatReader{gozxing.BarcodeFormat_EAN_8, oned.NewEAN8Reader()}, true
	case "UPC_A":
		return formatReader{gozxing.BarcodeFormat_UPC_A, oned.NewUPCAReader()}, true
	case "UPC_E":
		return formatReader{gozxing.BarcodeFormat_UPC_E, oned.NewUPCEReader()}, true
	case "ITF":
		return formatReader{gozxing.BarcodeFormat_ITF, oned.NewITFReader()}, true
	case "RSS_14":
		return formatReader{gozxing.BarcodeFormat_RSS_14, rss.NewRSS14Reader()}, true
	}
	return formatReader{}, false
}

func buildReaders(user *User) []formatReader {
	formats := effectiveBarcodeFormats(user)
	var readers []formatReader
	for _, name := range formats {
		if fr, ok := readerForFormat(strings.TrimSpace(strings.ToUpper(name))); ok {
			readers = append(readers, fr)
		}
	}
	return readers
}

func scanBarcodes(artifactID uint, ownerID *uint, user *User, data []byte) (codes []ScannedCode, err error) {
	readers := buildReaders(user)
	if len(readers) == 0 {
		return
	}

	img, imgErr := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if imgErr != nil {
		Log.Warnw("barcode scan: failed to decode image", "artifactID", artifactID, "error", imgErr)
		return
	}

	bmp, bmpErr := gozxing.NewBinaryBitmapFromImage(img)
	if bmpErr != nil {
		Log.Warnw("barcode scan: failed to create bitmap", "artifactID", artifactID, "error", bmpErr)
		return
	}

	seen := map[string]bool{}

	for _, fr := range readers {
		result, decErr := fr.reader.DecodeWithoutHints(bmp)
		fr.reader.Reset()
		if decErr != nil || result == nil {
			continue
		}
		key := result.GetBarcodeFormat().String() + ":" + result.GetText()
		if seen[key] {
			continue
		}
		seen[key] = true
		codes = append(codes, ScannedCode{
			ArtifactID: artifactID,
			OwnerID:    ownerID,
			Format:     result.GetBarcodeFormat().String(),
			Value:      result.GetText(),
		})
	}
	return
}

// --- Handlers ---

var GetArtifactScannedCodesOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/artifact/{id}/codes",
}

func GetArtifactScannedCodes(ctx context.Context, input *struct {
	ID uint `path:"id" example:"1" doc:"Artifact ID"`
}) (output *struct{ Body []ScannedCode }, err error) {
	_, _, userID, err := UserFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	artifact, err := GetArtifactFromDB(input.ID)
	if err != nil {
		return
	}
	if userID != nil && (artifact.OwnerID == nil || *artifact.OwnerID != *userID) {
		return nil, huma.Error404NotFound(errorArtifactNotFound)
	}

	codes, err := getScannedCodesForArtifact(input.ID)
	if err != nil {
		return
	}
	output = &struct{ Body []ScannedCode }{Body: codes}
	return
}

var GetRecordScannedCodesOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/record/{id}/codes",
}

func GetRecordScannedCodes(ctx context.Context, input *struct {
	ID uint `path:"id" example:"1" doc:"Record ID"`
}) (output *struct{ Body []ScannedCode }, err error) {
	username := UsernameFromContext(ctx)

	q := db.Model(&Record{}).Where("id = ?", input.ID)
	if username != "" {
		var u User
		if u, err = loadUser(username); err != nil {
			return
		}
		q = q.Where("owner_id = ?", u.ID)
	}
	var record Record
	if err = q.First(&record).Error; err != nil {
		return nil, huma.Error404NotFound(errorRecordNotFound)
	}

	codes, err := getScannedCodesForRecord(input.ID)
	if err != nil {
		return
	}
	output = &struct{ Body []ScannedCode }{Body: codes}
	return
}
