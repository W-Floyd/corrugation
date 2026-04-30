package backend

import (
	"github.com/danielgtaylor/huma/v2"
)

type EmptyOutput struct {
	Body struct{}
}

type PlaintextOutput struct {
	ContentType string `header:"Content-Type"`
	Body        string
}

type BytesOutput struct {
	ContentType        string `header:"Content-Type"`
	CacheControl       string `header:"Cache-Control"`
	ETag               string `header:"ETag"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

func RegisterHandlers(api huma.API) {

	huma.Register(api, ImportOperation, Import)
	huma.Register(api, ResetStoreOperation, ResetStore)

	huma.Register(api, ListRecordsOperation, ListRecords)
	huma.Register(api, GetRecordOperation, GetRecord)
	huma.Register(api, CreateRecordOperation, CreateRecord)
	huma.Register(api, UpdateRecordOperation, UpdateRecord)
	huma.Register(api, PatchRecordOperation, PatchRecord)
	huma.Register(api, DeleteRecordOperation, DeleteRecord)
	huma.Register(api, GetNextReferenceNumberOperation, GetNextReferenceNumber)

	huma.Register(api, VisualizeGraphRecordsOperation, VisualizeGraphRecords)
	huma.Register(api, FlushStaleEmbeddingsOperation, FlushStaleEmbeddings)
	huma.Register(api, GetEmbeddingProgressOperation, GetEmbeddingProgress)
	huma.Register(api, GetSearchEmbeddingProgressOperation, GetSearchEmbeddingProgress)

	huma.Register(api, CreateArtifactOperation, CreateArtifact)
	huma.Register(api, GetArtifactOperation, GetArtifact)
	huma.Register(api, DeleteArtifactOperation, DeleteArtifact)

	huma.Register(api, GetGlobalConfigOperation, GetGlobalConfig)
	huma.Register(api, PutGlobalConfigOperation, PutGlobalConfig)
	huma.Register(api, GetUserConfigOperation, GetUserConfig)
	huma.Register(api, PutUserConfigOperation, PutUserConfig)

	huma.Register(api, ListTagsOperation, ListTags)
	huma.Register(api, GetTagOperation, GetTag)
	huma.Register(api, CreateTagOperation, CreateTag)
	huma.Register(api, DeleteTagOperation, DeleteTag)

	// Image search endpoint
	huma.Register(api, SearchByImageOperation, SearchByImageHandler)
}
