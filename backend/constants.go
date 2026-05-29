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

	ollamaAddress      = "http://ollama:11434"
	ollamaVisionModel  = "moondream"
	ollamaNumCtx       = 4096
	ollamaImageMaxDim  = 512
	ollamaSuggestPrompt = `You are analyzing a household inventory item photo. Return a JSON object with these fields:
- "name": a short, descriptive name for the item using adjectives in this order: size, physical quality, age, shape, color, origin, material, purpose (string)
- "description": a brief description of the item including notable features (string)
- "quantity": estimated visible quantity as a whole number, or null if unclear (number or null)

Respond with valid JSON only. No explanation, no markdown.`

	embeddingSemaphore  = make(chan struct{}, 4)
	suggestionSemaphore = make(chan struct{}, 1)
)

func SetSuggestionConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	suggestionSemaphore = make(chan struct{}, n)
}

func SetInfinityConfig(address, textModel, imageModel, textQueryPrefix, textDocumentPrefix string) {
	infinityAddress = address
	infinityImageModel = imageModel
	infinityTextModel = textModel
	infinityTextQueryPrefix = textQueryPrefix
	infinityTextDocumentPrefix = textDocumentPrefix
}

func SetOllamaConfig(address, visionModel string, numCtx, imageMaxDim int, suggestPrompt string) {
	ollamaAddress = address
	ollamaVisionModel = visionModel
	if numCtx > 0 {
		ollamaNumCtx = numCtx
	}
	if imageMaxDim > 0 {
		ollamaImageMaxDim = imageMaxDim
	}
	if suggestPrompt != "" {
		ollamaSuggestPrompt = suggestPrompt
	}
}

func SetEmbeddingConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	embeddingSemaphore = make(chan struct{}, n)
}
