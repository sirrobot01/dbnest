// Mock data types and configurations

export type DatabaseEngine = "postgresql" | "mysql" | "mariadb" | "redis" | "sqlite";
export type DatabaseStatus = "running" | "stopped" | "error" | "creating";

export interface DatabaseInstance {
  id: string;
  name: string;
  engine: DatabaseEngine;
  version: string;
  status: DatabaseStatus;
  host: string;
  port: number;
  username: string;
  database: string;
  createdAt: string;
  storageUsed: number; // in MB
  storageLimit: number; // in MB
  memoryLimit: number; // in MB
  cpuLimit: number;
  connections: number;
  maxConnections: number;
}

export interface Backup {
  id: string;
  databaseId: string;
  databaseName: string;
  createdAt: string;
  size: number; // in MB
  status: "completed" | "in-progress" | "failed";
}

export const engineConfig: Record<string, {
  name: string;
  color: string;
  bgColor: string;
  borderColor: string;
  versions: string[];
  defaultPort: number;
}> = {
  postgresql: {
    name: "PostgreSQL",
    color: "text-foreground",
    bgColor: "bg-muted",
    borderColor: "border-border",
    versions: ["16", "15", "14", "13"],
    defaultPort: 5432,
  },
  mysql: {
    name: "MySQL",
    color: "text-foreground",
    bgColor: "bg-muted",
    borderColor: "border-border",
    versions: ["8.0", "8.4", "5.7"],
    defaultPort: 3306,
  },
  mariadb: {
    name: "MariaDB",
    color: "text-foreground",
    bgColor: "bg-muted",
    borderColor: "border-border",
    versions: ["11", "10.11", "10.6"],
    defaultPort: 3306,
  },
  redis: {
    name: "Redis",
    color: "text-foreground",
    bgColor: "bg-muted",
    borderColor: "border-border",
    versions: ["7", "7.2", "6"],
    defaultPort: 6379,
  },
  // Legacy support for existing sqlite databases
  sqlite: {
    name: "SQLite",
    color: "text-foreground",
    bgColor: "bg-muted",
    borderColor: "border-border",
    versions: ["3"],
    defaultPort: 0,
  },
};

export const statusConfig = {
  running: {
    label: "Running",
    color: "text-success-foreground",
    bgColor: "bg-success",
    dotColor: "bg-success-foreground",
  },
  stopped: {
    label: "Stopped",
    color: "text-muted-foreground",
    bgColor: "bg-muted",
    dotColor: "bg-muted-foreground",
  },
  error: {
    label: "Error",
    color: "text-destructive-foreground",
    bgColor: "bg-destructive",
    dotColor: "bg-destructive-foreground",
  },
  creating: {
    label: "Creating",
    color: "text-warning-foreground",
    bgColor: "bg-warning",
    dotColor: "bg-warning-foreground",
  },
};

export const mockDatabases: DatabaseInstance[] = [
  {
    id: "db-1",
    name: "production-api",
    engine: "postgresql",
    version: "16",
    status: "running",
    host: "localhost",
    port: 5432,
    username: "admin",
    database: "production_api",
    createdAt: "2025-12-01T10:00:00Z",
    storageUsed: 2048,
    storageLimit: 10240,
    memoryLimit: 512,
    cpuLimit: 1.0,
    connections: 12,
    maxConnections: 100,
  },
  {
    id: "db-2",
    name: "staging-backend",
    engine: "postgresql",
    version: "15",
    status: "running",
    host: "localhost",
    port: 5433,
    username: "staging_user",
    database: "staging_db",
    createdAt: "2025-12-10T14:30:00Z",
    storageUsed: 512,
    storageLimit: 5120,
    memoryLimit: 256,
    cpuLimit: 0.5,
    connections: 3,
    maxConnections: 50,
  },
  {
    id: "db-3",
    name: "analytics-warehouse",
    engine: "mysql",
    version: "8.0",
    status: "running",
    host: "localhost",
    port: 3306,
    username: "analytics",
    database: "analytics_data",
    createdAt: "2025-12-15T09:00:00Z",
    storageUsed: 4096,
    storageLimit: 20480,
    memoryLimit: 1024,
    cpuLimit: 2.0,
    connections: 8,
    maxConnections: 200,
  },
  {
    id: "db-4",
    name: "dev-testing",
    engine: "mysql",
    version: "8.0",
    status: "stopped",
    host: "localhost",
    port: 3307,
    username: "developer",
    database: "test_db",
    createdAt: "2025-12-20T16:45:00Z",
    storageUsed: 128,
    storageLimit: 1024,
    memoryLimit: 128,
    cpuLimit: 0.25,
    connections: 0,
    maxConnections: 20,
  },
  {
    id: "db-5",
    name: "mobile-app-cache",
    engine: "sqlite",
    version: "3",
    status: "running",
    host: "localhost",
    port: 0,
    username: "app",
    database: "cache.db",
    createdAt: "2025-12-25T11:20:00Z",
    storageUsed: 64,
    storageLimit: 512,
    memoryLimit: 64,
    cpuLimit: 0.1,
    connections: 1,
    maxConnections: 1,
  },
  {
    id: "db-6",
    name: "legacy-migration",
    engine: "postgresql",
    version: "13",
    status: "error",
    host: "localhost",
    port: 5434,
    username: "migrator",
    database: "legacy_import",
    createdAt: "2025-12-28T08:00:00Z",
    storageUsed: 1024,
    storageLimit: 8192,
    memoryLimit: 256,
    cpuLimit: 0.5,
    connections: 0,
    maxConnections: 50,
  },
];

export const mockBackups: Backup[] = [
  {
    id: "bk-1",
    databaseId: "db-1",
    databaseName: "production-api",
    createdAt: "2025-12-29T02:00:00Z",
    size: 1856,
    status: "completed",
  },
  {
    id: "bk-2",
    databaseId: "db-1",
    databaseName: "production-api",
    createdAt: "2025-12-28T02:00:00Z",
    size: 1824,
    status: "completed",
  },
  {
    id: "bk-3",
    databaseId: "db-3",
    databaseName: "analytics-warehouse",
    createdAt: "2025-12-29T03:00:00Z",
    size: 3840,
    status: "completed",
  },
];
