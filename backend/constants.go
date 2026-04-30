package backend

const (
	errorRecordNotFound            = "record not found"
	errorMoreRecordsThanExpected   = "more records than expected"
	errorArtifactNotFound          = "artifact not found"
	errorMoreArtifactsThanExpected = "more artifacts than expected"
	topLevelName                   = "World"
)

var (
	infinityAddress            = "http://infinity:8002"
	infinityImageModel         = "openai/clip-vit-large-patch14"
	infinityTextModel          = "BAAI/bge-large-en-v1.5"
	infinityTextQueryPrefix    = "Represent this sentence for searching relevant passages: "
	infinityTextDocumentPrefix = ""

	embeddingSemaphore = make(chan struct{}, 4)
)

func SetInfinityConfig(address, textModel, imageModel, textQueryPrefix, textDocumentPrefix string) {
	infinityAddress = address
	infinityImageModel = imageModel
	infinityTextModel = textModel
	infinityTextQueryPrefix = textQueryPrefix
	infinityTextDocumentPrefix = textDocumentPrefix
}

func SetEmbeddingConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	embeddingSemaphore = make(chan struct{}, n)
}
