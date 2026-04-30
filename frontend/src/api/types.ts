export interface Metadata {
  quantity: number | null;
  owner: string | null;
  lastModified: string | null;
  referenceNumber: string | null;
}

export interface Entity {
  id: number;
  name: string | null;
  description: string | null;
  artifacts: number[] | null;
  location: number;
  metadata: Metadata;
}

export interface Artifact {
  artifactid: number;
  path: string;
  image: boolean;
}

export type EntityCreate = Omit<Entity, "id">;

export interface BackendArtifactRef {
  ID: number;
}

export interface BackendRecord {
  ID: number;
  CreatedAt?: string;
  UpdatedAt?: string;
  ReferenceNumber?: string;
  Title?: string;
  Description?: string;
  Quantity?: number;
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

export function recordToEntity(r: BackendRecord): Entity {
  return {
    id: r.ID,
    name: r.Title ?? null,
    description: r.Description ?? null,
    artifacts: r.Artifacts?.map((a) => a.ID) ?? null,
    location: r.ParentID ?? 0,
    metadata: {
      quantity: r.Quantity ?? null,
      owner: null,
      referenceNumber: r.ReferenceNumber ?? null,
      lastModified: r.UpdatedAt ?? null,
    },
  };
}

export function entityToRecordBody(e: Entity | EntityCreate): RecordBody {
  return {
    Title: e.name ?? null,
    ReferenceNumber: e.metadata.referenceNumber,
    Description: e.description,
    Quantity: e.metadata.quantity ?? undefined,
    ParentID: e.location || undefined,
    Artifacts: e.artifacts ?? undefined,
  };
}
