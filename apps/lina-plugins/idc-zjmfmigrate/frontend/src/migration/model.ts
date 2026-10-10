export interface MigrationPreflightInput {
  charset: string;
  currencies: string[];
  sourceSystem: string;
  sourceVersion: string;
  timezone: string;
}

export interface MigrationPreflightResult {
  blockers: string[];
  capabilities: string[];
  sourceSystem: string;
  sourceVersion: string;
  supported: boolean;
  warnings: string[];
}

export interface MigrationBatch {
  batchNo: string;
  id: number;
  status: string;
}

export interface MigrationRunResult {
  blockers: string[];
  id: number;
  mapped: number;
  status: string;
}

export interface MigrationConflict {
  decision: string;
  id: number;
  objectType: string;
  sourceId: string;
  summary: string;
}

export interface MigrationReport {
  blockers: string[];
  byType: Record<string, string>;
  id: number;
  status: string;
  warnings: string[];
}
