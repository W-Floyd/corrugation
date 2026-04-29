package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/boxes-ltd/imaging"
	"github.com/chai2010/webp"
	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

type ArtifactInterface interface {
	Store(ctx context.Context, file huma.FormFile) (err error)
	GetOriginalContents() (output *[]byte, err error)
	GetSmallPreviewContents() (output *[]byte, err error)
	GetLargePreviewContents() (output *[]byte, err error)
	GetOriginalFilename() (output string, err error)
	GetContentType() (output string, err error)
	GetID() (output uint)
	GenerateEmbeddings(ctx context.Context) (err error)
}

type Artifact struct {
	gorm.Model

	Data             *[]byte `json:",omitempty"`
	OriginalFilename *string `json:",omitempty"`
	ContentType      *string `json:",omitempty"`

	SmallPreviewID *uint     `json:"-" gorm:"index"`
	SmallPreview   *Artifact `json:"-" gorm:"foreignKey:SmallPreviewID"`
	LargePreviewID *uint     `json:"-" gorm:"index"`
	LargePreview   *Artifact `json:"-" gorm:"foreignKey:LargePreviewID"`

	RecordID *uint `json:",omitempty" gorm:"index"`
	OwnerID  *uint `json:",omitempty" gorm:"index"`
}

func GetArtifactFromDB(ID uint) (artifact Artifact, err error) {
	var artifacts []Artifact
	artifacts, err = gorm.G[Artifact](db).Where("id = ?", ID).Preload("SmallPreview", nil).Preload("LargePreview", nil).Find(dbCtx)
	if err != nil {
		return
	} else if len(artifacts) > 1 {
		err = huma.Error500InternalServerError(errorMoreArtifactsThanExpected)
		return
	} else if len(artifacts) == 0 {
		err = huma.Error404NotFound(errorArtifactNotFound)
		return
	}
	artifact = artifacts[0]
	return
}

func (a *Artifact) GetInterface() (output ArtifactInterface, err error) {
	if a.ContentType == nil {
		err = huma.Error415UnsupportedMediaType("empty content type")
	} else if strings.HasPrefix(*a.ContentType, "image/") {
		i := Image(*a)
		output = &i
	} else {
		switch filepath.Ext(*a.OriginalFilename) {
		case ".png", ".jpeg", ".jpg", ".webp":
			i := Image(*a)
			output = &i
		default:
			err = huma.Error415UnsupportedMediaType("unsupported content type " + *a.ContentType)
			return
		}

	}
	return
}

type Image Artifact

func (i *Image) Store(ctx context.Context, file huma.FormFile) (err error) {

	b, err := io.ReadAll(file)
	if err != nil {
		Log.Error(err)
		return
	}

	i.Data = &b
	i.OriginalFilename = &file.Filename
	i.ContentType = &file.ContentType

	err = i.ComputePreviews()
	if err != nil {
		Log.Error(err)
		return
	}

	// Get owner_id for the artifact
	uc, _ := loadUser(UsernameFromContext(ctx))
	var ownerID *uint
	if uc.ID > 0 {
		ownerID = &uc.ID
	}

	a := Artifact(*i)
	a.OwnerID = ownerID

	err = gorm.G[Artifact](db).Create(dbCtx, &a)
	if err != nil {
		Log.Error(err)
		return
	}

	*i = Image(a)

	_, imageModel, _, _ := effectiveInfinityConfig(uc)
	EnqueueEmbeddingJob(JobTypeArtifact, i.ID, ownerID, UsernameFromContext(ctx), imageModel, "store")

	return
}

func (i *Image) computePreview(size int, quality float32) (o *Artifact, err error) {
	preview, err := i.ComputePreview(size, quality)
	if err != nil {
		return
	}
	ct := http.DetectContentType(*preview)
	o = &Artifact{
		Data:        preview,
		ContentType: &ct,
	}
	err = gorm.G[Artifact](db).Create(dbCtx, o)
	return
}

func (i *Image) computeSmallPreview() (o *Artifact, err error) {
	o, err = i.computePreview(625*1000, 70)
	if err != nil {
		return
	}
	i.SmallPreview = o
	i.SmallPreviewID = &o.ID
	if i.ID > 0 {
		var n int
		// n, err := gorm.G[Artifact](db).Where("id = ?", i.ID).Update(dbCtx, "small_preview", *o)
		// if err != nil {
		// 	return
		// } else if n != 1 {
		// 	err = errors.New("affected " + strconv.Itoa(n) + " image small_preview")
		// 	return
		// }
		n, err = gorm.G[Artifact](db).Where("id = ?", i.ID).Update(dbCtx, "small_preview_id", o.ID)
		if err != nil {
			return
		} else if n != 1 {
			err = errors.New("affected " + strconv.Itoa(n) + " image small_preview_id")
			return
		}
	}
	return
}

func (i *Image) computeLargePreview() (o *Artifact, err error) {
	o, err = i.computePreview(1250*1000, 75)
	if err != nil {
		return
	}
	i.LargePreview = o
	i.LargePreviewID = &o.ID
	if i.ID > 0 {
		var n int
		// n, err := gorm.G[Artifact](db).Where("id = ?", i.ID).Update(dbCtx, "large_preview", *o)
		// if err != nil {
		// 	return
		// } else if n != 1 {
		// 	err = errors.New("affected " + strconv.Itoa(n) + " image large_preview")
		// 	return
		// }
		n, err = gorm.G[Artifact](db).Where("id = ?", i.ID).Update(dbCtx, "large_preview_id", o.ID)
		if err != nil {
			return
		} else if n != 1 {
			err = errors.New("affected " + strconv.Itoa(n) + " image large_preview_id")
			return
		}
	}
	return
}

func (i *Image) ComputePreviews() (err error) {

	_, err = i.computeSmallPreview()
	if err != nil {
		return
	}
	_, err = i.computeLargePreview()

	return
}

func (i *Image) ComputePreview(maximumPixelCount int, quality float32) (output *[]byte, err error) {

	img, err := imaging.Decode(bytes.NewBuffer(*i.Data), imaging.AutoOrientation(true))
	if err != nil {
		return
	}

	if img.Bounds().Dx()*img.Bounds().Dy() > maximumPixelCount {
		ratio := float64(img.Bounds().Dx()) / float64(img.Bounds().Dy())
		scaler := math.Sqrt(float64(maximumPixelCount) / (ratio * float64(img.Bounds().Dy()*img.Bounds().Dy())))
		img = imaging.Resize(img, int(float64(img.Bounds().Dx())*scaler), int(float64(img.Bounds().Dy())*scaler), imaging.Lanczos)
	}

	buf := new(bytes.Buffer)

	err = webp.Encode(buf, img, &webp.Options{Quality: quality})
	if err != nil {
		return
	}

	var o []byte
	o, err = io.ReadAll(buf)
	if err != nil {
		return
	}
	output = &o
	return
}

func (i *Image) GetOriginalContents() (output *[]byte, err error) {
	if i.Data == nil || len(*i.Data) == 0 {
		err = errors.New("no data in image")
		return
	}
	output = i.Data
	return
}
func (i *Image) GetSmallPreviewContents() (output *[]byte, err error) {
	if i.SmallPreview != nil && i.SmallPreview.Data != nil && len(*i.SmallPreview.Data) > 0 {
		output = i.SmallPreview.Data
		return
	}
	if i.SmallPreviewID != nil {
		a, fetchErr := GetArtifactFromDB(*i.SmallPreviewID)
		if fetchErr != nil {
			return nil, fetchErr
		}
		i.SmallPreview = &a
		output = a.Data
		return
	}
	_, err = i.computeSmallPreview()
	if err != nil {
		return
	}
	output = i.SmallPreview.Data
	return
}
func (i *Image) GetLargePreviewContents() (output *[]byte, err error) {
	if i.LargePreview != nil && i.LargePreview.Data != nil && len(*i.LargePreview.Data) > 0 {
		output = i.LargePreview.Data
		return
	}
	if i.LargePreviewID != nil {
		a, fetchErr := GetArtifactFromDB(*i.LargePreviewID)
		if fetchErr != nil {
			return nil, fetchErr
		}
		i.LargePreview = &a
		output = a.Data
		return
	}
	_, err = i.computeLargePreview()
	if err != nil {
		return
	}
	output = i.LargePreview.Data
	return
}
func (i *Image) GetOriginalFilename() (output string, err error) {
	if i.OriginalFilename == nil {
		err = errors.New("no filename in image")
		return
	}
	output = *i.OriginalFilename
	return
}
func (i *Image) GetContentType() (output string, err error) {
	if i.ContentType == nil {
		err = errors.New("no content type associated with image")
		return
	}
	output = *i.ContentType
	return
}
func (i *Image) GetID() (output uint) {
	return i.ID
}

type artifactEmbedding struct {
	embedding []float64
	recordID  *uint
}

func GetArtifactEmbeddings(ctx context.Context, artifactRecordMap map[uint]*uint) (e map[uint]*artifactEmbedding, partial bool, err error) {

	artifactIDs := make([]uint, 0, len(artifactRecordMap))
	for id := range artifactRecordMap {
		artifactIDs = append(artifactIDs, id)
	}

	loadEmbeddings := func(ids []uint) error {
		embeddings, fetchErr := gorm.G[Embedding](db).Where("artifact_id IN ? AND embed_model = ?", ids, infinityImageModel).Find(dbCtx)
		if fetchErr != nil {
			return fetchErr
		}
		for _, emb := range embeddings {
			if emb.ArtifactID == nil || e[*emb.ArtifactID] != nil {
				continue
			}
			var vec []float64
			if cached, ok := embeddingsCache.Load(emb.Hash); ok {
				vec = cached.(Embeddings)
			} else {
				if jsonErr := json.Unmarshal(emb.Data, &vec); jsonErr != nil {
					continue
				}
				embeddingsCache.Store(emb.Hash, Embeddings(vec))
			}
			e[*emb.ArtifactID] = &artifactEmbedding{
				embedding: vec,
				recordID:  artifactRecordMap[*emb.ArtifactID],
			}
		}
		return nil
	}

	e = map[uint]*artifactEmbedding{}
	if err = loadEmbeddings(artifactIDs); err != nil {
		return
	}

	embeddedIDs := map[uint]bool{}
	for id := range e {
		embeddedIDs[id] = true
	}

	enqueuedIDs := generateMissingArtifactEmbeddings(ctx, artifactIDs, embeddedIDs, "search")

	partial = len(enqueuedIDs) > 0

	return
}

// generateMissingArtifactEmbeddings enqueues embedding jobs for artifact IDs not in embeddedIDs.
// Returns the IDs that were enqueued.
func generateMissingArtifactEmbeddings(ctx context.Context, artifactIDs []uint, embeddedIDs map[uint]bool, source string) []uint {
	uc, _ := loadUser(UsernameFromContext(ctx))
	_, imageModel, _, _ := effectiveInfinityConfig(uc)
	var ownerID *uint
	if uc.ID > 0 {
		ownerID = &uc.ID
	}
	username := UsernameFromContext(ctx)
	var enqueued []uint
	for _, id := range artifactIDs {
		if !embeddedIDs[id] {
			EnqueueEmbeddingJob(JobTypeArtifact, id, ownerID, username, imageModel, source)
			enqueued = append(enqueued, id)
		}
	}
	return enqueued
}
