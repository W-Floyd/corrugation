package backend

import (
	"context"

	"gorm.io/gorm"
)

func BackfillEmbeddings() {
	backfillArtifactOwners()
	backfillRecordEmbeddings()
	backfillArtifactEmbeddings()
}

func backfillRecordEmbeddings() {
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
		ctx := context.WithValue(dbCtx, usernameContextKey, u.Username)
		backfillRecordEmbeddingsForUser(ctx, textModel, docPrefix, ownerRecords)
	}
}

func backfillRecordEmbeddingsForUser(ctx context.Context, textModel, docPrefix string, records []Record) {
	recordIDs := make([]uint, len(records))
	for i, r := range records {
		recordIDs[i] = r.ID
	}

	embeddings, err := gorm.G[Embedding](db).Where("record_id IN ? AND embed_model = ?", recordIDs, textModel).Find(dbCtx)
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
}

func backfillArtifactOwners() {
	var artifacts []Artifact
	err := db.Select("artifacts.id, artifacts.record_id, records.owner_id").
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
}

func backfillArtifactEmbeddings() {
	embeddings, err := gorm.G[Embedding](db).Where("artifact_id IS NOT NULL AND embed_model = ?", infinityImageModel).Find(dbCtx)
	if err != nil {
		Log.Errorw("backfill: failed to fetch artifact embeddings", "error", err)
		return
	}
	embeddedIDs := map[uint]bool{}
	for _, e := range embeddings {
		if e.ArtifactID != nil {
			embeddedIDs[*e.ArtifactID] = true
		}
	}

	artifacts, err := gorm.G[Artifact](db).Select("id").Find(dbCtx)
	if err != nil {
		Log.Errorw("backfill: failed to fetch artifacts", "error", err)
		return
	}
	artifactIDs := make([]uint, len(artifacts))
	for i, a := range artifacts {
		artifactIDs[i] = a.ID
	}

	generateMissingArtifactEmbeddings(dbCtx, artifactIDs, embeddedIDs, "backfill")
}
