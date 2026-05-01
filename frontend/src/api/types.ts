export interface BackendRecord {
  ID: number;
  CreatedAt?: string;
  UpdatedAt?: string;
  ReferenceNumber?: string | null;
  Title?: string | null;
  Description?: string | null;
  Quantity?: number | null;
  Artifacts?: BackendArtifactRef[];
  ParentID?: number;
  SearchConfidenceImage?: number;
  SearchConfidenceText?: number;
}

export interface RecordBody {
  Title?: string | null;
  ReferenceNumber?: string | null;
  Description?: string | null;
  Quantity?: number | null;
  ParentID?: number | null;
  Artifacts?: number[];
}

export interface BackendArtifactRef {
  ID: number;
}
