package backend

import (
	"context"

	"gorm.io/gorm"
)

func Backfill(flags BackfillFlags) (err error) {
	if err = clearEmbeddingJobs(); err != nil {
		return
	}
	if flags.ArtifactOwners {
		if err = backfillArtifactOwners(); err != nil {
			return
		}
	}
	if flags.RecordEmbeddings {
		if err = backfillRecordEmbeddings(); err != nil {
			return
		}
	}
	if flags.ArtifactEmbeddings {
		if err = backfillArtifactEmbeddings(); err != nil {
			return
		}
	}
	if flags.Suggestions {
		if err = backfillArtifactSuggestions(); err != nil {
			return
		}
	}
	return
}

func clearEmbeddingJobs() (err error) {
	// Reset interrupted processing jobs back to pending
	db.Model(&EmbeddingJob{}).Where("status = ?", JobStatusProcessing).Update("status", JobStatusPending)

	var jobs []EmbeddingJob
	tx := db.Where("status = ?", JobStatusPending).Find(&jobs)
	if tx.Error != nil {
		return tx.Error
	}

	return
}

func backfillRecordEmbeddings() (err error) {
	records, _, err := GetRecords(dbCtx, nil, nil, nil, nil, nil, []string{"id", "title", "reference_number", "description", "owner_id"})
	if err != nil {
		Log.Errorw("backfill: failed to fetch records", "error", err)
		return
	}

	// Collect unique owner IDs and load their User rows.
	ownerIDSet := map[uint]bool{}
	for _, r := range records {
		if r.OwnerID != nil {
			ownerIDSet[*r.OwnerID] = true
		}
	}
	ownerIDs := make([]uint, 0, len(ownerIDSet))
	for id := range ownerIDSet {
		ownerIDs = append(ownerIDs, id)
	}
	var owners []User
	if len(ownerIDs) > 0 {
		err = db.Where("id IN ?", ownerIDs).Find(&owners).Error
		if err != nil {
			Log.Errorw("backfill: failed to fetch owners", "owner_ids", ownerIDs, "error", err)
			return
		}
	}
	userByID := map[uint]User{}
	for _, u := range owners {
		userByID[u.ID] = u
	}

	// Group records by owner ID (nil owner = global defaults).
	type ownerKey struct {
		valid bool
		id    uint
	}
	byOwner := map[ownerKey][]Record{}
	for _, r := range records {
		var key ownerKey
		if r.OwnerID != nil {
			key = ownerKey{true, *r.OwnerID}
		}
		byOwner[key] = append(byOwner[key], r)
	}

	for key, ownerRecords := range byOwner {
		var u User
		if key.valid {
			u = userByID[key.id]
		}
		textModel, _, _, docPrefix := effectiveInfinityConfig(&u)
		maxDims := effectiveMaxEmbeddingDimensions(&u)
		ctx := context.WithValue(dbCtx, usernameContextKey, u.Username)
		backfillRecordEmbeddingsForUser(ctx, textModel, docPrefix, maxDims, ownerRecords)
	}
	return
}

func backfillRecordEmbeddingsForUser(ctx context.Context, textModel, docPrefix string, maxDims *uint, records []Record) (err error) {
	recordIDs := make([]uint, len(records))
	for i, r := range records {
		recordIDs[i] = r.ID
	}

	q := gorm.G[Embedding](db).Where("record_id IN ? AND embed_model = ?", recordIDs, textModel)
	if maxDims != nil {
		q = q.Where("dimensions = ?", *maxDims)
	}
	embeddings, err := q.Find(dbCtx)
	if err != nil {
		Log.Errorw("backfill: failed to fetch embeddings", "model", textModel, "error", err)
		return
	}
	storedHash := map[uint]string{}
	for _, e := range embeddings {
		if e.RecordID != nil {
			storedHash[*e.RecordID] = e.Hash
		}
	}

	embeddedIDs := map[uint]bool{}
	for _, r := range records {
		text := recordEmbeddingText(r)
		if text == "" {
			continue
		}
		if storedHash[r.ID] == InputHash(docPrefix+text) {
			embeddedIDs[r.ID] = true
		}
	}

	generateMissingRecordEmbeddings(ctx, recordIDs, embeddedIDs, "backfill")
	return
}

func backfillArtifactOwners() (err error) {
	var artifacts []Artifact
	err = db.Select("artifacts.id, artifacts.record_id, records.owner_id").
		Joins("JOIN records ON records.id = artifacts.record_id AND records.owner_id IS NOT NULL").
		Where("artifacts.owner_id IS NULL AND artifacts.record_id IS NOT NULL").
		Find(&artifacts).Error
	if err != nil {
		Log.Errorw("backfill: failed to fetch ownerless artifacts", "error", err)
		return
	}
	if len(artifacts) == 0 {
		return
	}

	for _, a := range artifacts {
		if err := db.Model(&Artifact{}).Where("id = ?", a.ID).Update("owner_id", a.OwnerID).Error; err != nil {
			Log.Errorw("backfill: failed to update artifact owner", "artifact_id", a.ID, "owner_id", a.OwnerID, "error", err)
		}
	}
	Log.Infow("backfill: assigned owners to ownerless artifacts", "count", len(artifacts))
	return
}

func backfillArtifactEmbeddings() (err error) {
	artifacts, err := gorm.G[Artifact](db).Select("id, owner_id").Find(dbCtx)
	if err != nil {
		Log.Errorw("backfill: failed to fetch artifacts", "error", err)
		return
	}

	ownerIDSet := map[uint]bool{}
	for _, a := range artifacts {
		if a.OwnerID != nil {
			ownerIDSet[*a.OwnerID] = true
		}
	}
	ownerIDs := make([]uint, 0, len(ownerIDSet))
	for id := range ownerIDSet {
		ownerIDs = append(ownerIDs, id)
	}
	var owners []User
	if len(ownerIDs) > 0 {
		if err = db.Where("id IN ?", ownerIDs).Find(&owners).Error; err != nil {
			Log.Errorw("backfill: failed to fetch artifact owners", "error", err)
			return
		}
	}
	userByID := map[uint]User{}
	for _, u := range owners {
		userByID[u.ID] = u
	}

	type ownerKey struct {
		valid bool
		id    uint
	}
	byOwner := map[ownerKey][]uint{}
	for _, a := range artifacts {
		var key ownerKey
		if a.OwnerID != nil {
			key = ownerKey{true, *a.OwnerID}
		}
		byOwner[key] = append(byOwner[key], a.ID)
	}

	for key, ids := range byOwner {
		var u User
		if key.valid {
			u = userByID[key.id]
		}
		_, imageModel, _, _ := effectiveInfinityConfig(&u)
		maxDims := effectiveMaxEmbeddingDimensions(&u)
		ctx := context.WithValue(dbCtx, usernameContextKey, u.Username)

		q := gorm.G[Embedding](db).Where("artifact_id IN ? AND embed_model = ?", ids, imageModel)
		if maxDims != nil {
			q = q.Where("dimensions = ?", *maxDims)
		}
		embeddings, fetchErr := q.Find(dbCtx)
		if fetchErr != nil {
			Log.Errorw("backfill: failed to fetch artifact embeddings", "error", fetchErr)
			err = fetchErr
			return
		}
		embeddedIDs := map[uint]bool{}
		for _, e := range embeddings {
			if e.ArtifactID != nil {
				embeddedIDs[*e.ArtifactID] = true
			}
		}
		generateMissingArtifactEmbeddings(ctx, ids, embeddedIDs, "backfill")
	}
	return
}
