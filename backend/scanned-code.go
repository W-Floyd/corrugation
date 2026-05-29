package backend

type ScannedCode struct {
	Model
	ArtifactID uint   `json:"artifactId" gorm:"index"`
	OwnerID    *uint  `json:"ownerId,omitempty" gorm:"index"`
	Format     string `json:"format"`
	Value      string `json:"value" gorm:"index"`
}

func saveScannedCodes(codes []ScannedCode) error {
	if len(codes) == 0 {
		return nil
	}
	return db.Create(&codes).Error
}

func getScannedCodesForArtifact(artifactID uint) ([]ScannedCode, error) {
	var codes []ScannedCode
	err := db.Where("artifact_id = ?", artifactID).Find(&codes).Error
	return codes, err
}

func getScannedCodesForRecord(recordID uint) ([]ScannedCode, error) {
	var codes []ScannedCode
	err := db.Joins("JOIN artifacts ON artifacts.id = scanned_codes.artifact_id").
		Where("artifacts.record_id = ?", recordID).
		Where("artifacts.deleted_at IS NULL").
		Find(&codes).Error
	return codes, err
}

func deleteScannedCodesForArtifact(artifactID uint) error {
	return db.Where("artifact_id = ?", artifactID).Delete(&ScannedCode{}).Error
}
