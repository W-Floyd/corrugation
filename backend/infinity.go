package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
)

type infinityEmbeddingsRequest struct {
	Model          string   `json:"model"`
	EncodingFormat string   `json:"encoding_format"`
	Input          []string `json:"input"`
	Modality       string   `json:"modality"`
	Dimensions     uint     `json:"dimensions,omitempty"`
}

type infinityEmbeddingsReponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     uint      `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens uint `json:"prompt_tokens"`
		TotalTokens  uint `json:"total_tokens"`
	} `json:"usage"`
	ID      string `json:"id"`
	Created int64  `json:"created"`
}

type Embeddings []float64

var embeddingsCache sync.Map // hash string → Embeddings

func (i *infinityEmbeddingsRequest) GenerateEmbeddings() (e Embeddings, err error) {

	b, err := json.Marshal(*i)
	if err != nil {
		return
	}

	c := http.Client{}
	resp, err := c.Post(infinityAddress+"/embeddings", "application/json", bytes.NewBuffer(b))
	if err != nil {
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.Join(errors.New(string(respBody)), errors.New("http error "+strconv.Itoa(resp.StatusCode)+" when submitting data to Infinity backend"))
		return
	}

	var infinityResponse infinityEmbeddingsReponse

	err = json.Unmarshal(respBody, &infinityResponse)
	if err != nil {
		return
	}

	e = infinityResponse.Data[0].Embedding

	return

}

func GenerateTextDocumentEmbeddingsCtx(ctx context.Context, input string) (e Embeddings, fullInput string, err error) {
	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, _, _, docPrefix := effectiveInfinityConfig(user)
	maxDims := effectiveMaxEmbeddingDimensions(user)
	fullInput = docPrefix + input
	e, err = generateTextEmbeddings(fullInput, textModel, maxDims)
	return
}

func GenerateTextQueryEmbeddingsCtx(ctx context.Context, input string) (embeddings Embeddings, err error) {
	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, _, queryPrefix, _ := effectiveInfinityConfig(user)
	maxDims := effectiveMaxEmbeddingDimensions(user)
	embeddings, err = generateTextEmbeddings(queryPrefix+input, textModel, maxDims)
	return
}

func GenerateImageQueryEmbeddingsCtx(ctx context.Context, input string) (embeddings Embeddings, err error) {
	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	_, imageModel, queryPrefix, _ := effectiveInfinityConfig(user)
	maxDims := effectiveMaxEmbeddingDimensions(user)
	// Text query searching images: use text modality for cross-modal search
	embeddings, err = generateTextEmbeddings(queryPrefix+input, imageModel, maxDims)
	return
}

func generateTextEmbeddings(input, model string, maxDims *uint) (e Embeddings, err error) {
	req := infinityEmbeddingsRequest{
		Model:          model,
		EncodingFormat: "float",
		Input:          []string{input},
		Modality:       "text",
	}
	if maxDims != nil {
		req.Dimensions = *maxDims
	}
	e, err = req.GenerateEmbeddings()
	return
}

func generateImageEmbeddings(input, model string, maxDims *uint) (e Embeddings, err error) {
	req := infinityEmbeddingsRequest{
		Model:          model,
		EncodingFormat: "float",
		Input:          []string{input},
		Modality:       "image",
	}
	if maxDims != nil {
		req.Dimensions = *maxDims
	}
	e, err = req.GenerateEmbeddings()
	return
}

func (i *Image) GenerateEmbeddings(ctx context.Context) (err error) {
	if i.ID == 0 {
		err = errors.New("artifact must be persisted before generating embeddings")
		return
	}
	if i.Data == nil || len(*i.Data) == 0 {
		err = errors.New("no data in image")
		return
	}

	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	_, imageModel, _, _ := effectiveInfinityConfig(user)
	maxDims := effectiveMaxEmbeddingDimensions(user)

	base64Image := base64.StdEncoding.EncodeToString(*i.Data)
	base64Image = "data:" + http.DetectContentType(*i.Data) + ";base64," + base64Image

	e, err := generateImageEmbeddings(base64Image, imageModel, maxDims)
	if err != nil {
		return
	}

	id := i.ID
	err = saveEmbedding(nil, &id, e, imageModel, base64Image)
	if err == nil {
		Log.Infof("embedding: artifact %d indexed with model %s", id, imageModel)
	}
	return
}

func AverageEmbeddings(vecs []Embeddings) (Embeddings, error) {
	if len(vecs) == 0 {
		return nil, errors.New("no embeddings to average")
	}
	dim := len(vecs[0])
	for _, v := range vecs {
		if len(v) != dim {
			return nil, errors.New("embedding dimension mismatch")
		}
	}
	avg := make(Embeddings, dim)
	n := float64(len(vecs))
	for _, v := range vecs {
		for i, x := range v {
			avg[i] += x / n
		}
	}
	return avg, nil
}

func InputHash(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return string(h.Sum(nil))
}

func (e *Embeddings) MarshalEmbeddings(input string) (hash string, data []byte, err error) {
	data = make([]byte, len(*e)*8)
	for i, f := range *e {
		binary.NativeEndian.PutUint64(data[i*8:], math.Float64bits(f))
	}

	hash = InputHash(input)
	embeddingsCache.LoadOrStore(hash, *e)
	return
}

func UnmarshalEmbeddings(data []byte) (Embeddings, error) {
	if len(data)%8 != 0 {
		return nil, errors.New("embedding data length is not a multiple of 8")
	}
	e := make(Embeddings, len(data)/8)
	for i := range e {
		e[i] = math.Float64frombits(binary.NativeEndian.Uint64(data[i*8:]))
	}
	return e, nil
}
