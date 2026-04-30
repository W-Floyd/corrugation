package backend

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

type TagOutput struct {
	Body Tag
}

type TagsOutput struct {
	Body []Tag
}

var GetTagOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/tag/{id}",
}

func GetTag(ctx context.Context, input *struct {
	Title string `path:"title" example:"Electrical" doc:"Title of tag to get"`
}) (output *TagOutput, err error) {
	var tags []Tag
	tags, err = GetTags(&input.Title, true)
	if err != nil {
		return
	}
	output = &TagOutput{
		Body: tags[0],
	}
	return
}

var ListTagsOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/tags",
}

func ListTags(ctx context.Context, input *struct{}) (output *TagsOutput, err error) {
	var tags []Tag
	tags, err = GetTags(nil, false)
	output = &TagsOutput{
		Body: tags,
	}
	return
}

var CreateTagOperation = huma.Operation{
	Method: http.MethodPost,
	Path:   "/api/tag",
}

func CreateTag(ctx context.Context, input *struct {
	Body TagInput
}) (output *TagOutput, err error) {
	tag, err := input.Body.Convert()
	if err != nil {
		return
	}

	err = gorm.G[Tag](db).Create(dbCtx, &tag)
	output = &TagOutput{
		Body: tag,
	}
	return
}

var DeleteTagOperation = huma.Operation{
	Method: http.MethodDelete,
	Path:   "/api/tag/{id}",
}

func DeleteTag(ctx context.Context, input *struct {
	Title string `path:"title" example:"Electrical" doc:"Title of tag to delete"`
}) (output *EmptyOutput, err error) {

	chain := gorm.G[Tag](db).Where("title = ?", input.Title)

	tags, err := chain.Find(dbCtx)
	if err != nil {
		return
	}
	if len(tags) == 0 {
		err = huma.Error404NotFound(errorTagNotFound)
		return
	} else if len(tags) > 1 {
		err = huma.Error500InternalServerError(errorMoreTagsThanExpected)
		return
	}

	rowsAffected, err := gorm.G[Tag](db).Where("title = ?", input.Title).Delete(dbCtx)
	if err != nil {
		return
	}
	if rowsAffected == 0 {
		err = huma.Error404NotFound(errorTagNotFound)
	} else if rowsAffected > 1 {
		err = huma.Error500InternalServerError(errorMoreTagsThanExpected)
		return
	}
	output = &EmptyOutput{}
	return
}

var VisualizeGraphTagsOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/tags/visualize",
}
