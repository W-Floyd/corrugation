package backend

import (
	"time"

	"gorm.io/gorm"
)

type TagInput struct {
	Title string `required:"true" example:"Electrical" doc:"Title of tag being created"`
	Color string `required:"false" example:"#ff0000" doc:"Hex color of tag being created"`
}

type Tag struct {
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Title string `gorm:"primarykey"`
	Color string

	Records []*Record `gorm:"many2many:record_tags;"`
}

func (i *TagInput) Convert() (o Tag, err error) {
	o.Title = i.Title
	o.Color = i.Color
	return
}

func (record *Tag) PrettyString() (output string) {
	output = record.Title
	return
}
