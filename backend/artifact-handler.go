package backend

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

var CreateArtifactOperation = huma.Operation{
	Method: http.MethodPost,
	Path:   "/api/artifact",
}

func CreateArtifact(ctx context.Context, input *struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true"`
	}]
}) (output *UIntOutput, err error) {
	f := input.RawBody.Data().File

	var a ArtifactInterface

	if strings.HasPrefix(f.ContentType, "image/") {
		a = &Image{}
	} else {
		switch filepath.Ext(f.Filename) {
		case ".png", ".jpeg", ".jpg", ".webp":
			a = &Image{}
		default:
			err = huma.Error415UnsupportedMediaType("unsupported media type " + f.ContentType)
			return
		}
	}

	err = a.Store(ctx, f)
	if err != nil {
		Log.Error(err)
		return
	}

	Broadcast()

	output = &UIntOutput{
		Body: a.GetID(),
	}

	return
}

var GetArtifactOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/artifact/{id}",
}

func GetArtifact(ctx context.Context, input *struct {
	conditional.Params
	ID       uint `path:"id" example:"1" doc:"Artifact ID to get"`
	Original bool `query:"original" doc:"Return the original file instead of the preview" required:"false"`
}) (output *BytesOutput, err error) {
	username, user, userID, err := UserFromContext(ctx)
	if err != nil {
		return
	}

	artifact, err := GetArtifactFromDB(input.ID)
	if err != nil {
		return
	}

	_, imageModel, _, _ := effectiveInfinityConfig(user)
	EnqueueEmbeddingJob(JobTypeArtifact, artifact.ID, userID, username, imageModel, "search")

	etag := fmt.Sprintf(`"%d"`, artifact.UpdatedAt.UnixMilli())

	if input.HasConditionalParams() {
		if err = input.PreconditionFailed(etag, artifact.UpdatedAt); err != nil {
			return
		}
	}

	i, err := artifact.GetInterface()

	var ob *[]byte
	if input.Original {
		ob, err = i.GetOriginalContents()
	} else {
		ob, err = i.GetSmallPreviewContents()
	}
	if err != nil {
		return
	}

	output = &BytesOutput{}
	output.Body = *ob
	output.CacheControl = "public, max-age=604800"
	output.ETag = etag

	if input.Original {
		ct, ctErr := i.GetContentType()
		if ctErr == nil && ct != "" {
			output.ContentType = ct
		} else {
			output.ContentType = http.DetectContentType(output.Body)
		}
		fn, fnErr := i.GetOriginalFilename()
		if fnErr == nil && fn != "" {
			output.ContentDisposition = fmt.Sprintf(`attachment; filename=%q`, filepath.Base(fn))
		}
	} else {
		output.ContentType = http.DetectContentType(output.Body)
	}

	return
}

var DeleteArtifactOperation = huma.Operation{
	Method: http.MethodDelete,
	Path:   "/api/artifact/{id}",
}

func DeleteArtifact(ctx context.Context, input *struct {
	ID uint `path:"id" example:"1" doc:"Artifact ID to delete"`
}) (*EmptyOutput, error) {
	err := DeleteArtifactFromDB(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	Broadcast()

	return &EmptyOutput{}, nil
}
