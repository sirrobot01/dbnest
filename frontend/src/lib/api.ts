// API client for DBnest backend
const API_BASE = '/api/v1';

export interface DatabaseInstance {
    id: string;
    name: string;
    engine: 'postgresql' | 'mysql' | 'mariadb' | 'redis';
    version: string;
    status: 'running' | 'stopped' | 'error' | 'creating';
    host: string;
    port: number;
    username: string;
    database: string;
    containerId?: string;
    createdAt: string;
    storageUsed: number;
    storageLimit: number;
    memoryLimit: number;
    cpuLimit: number;
    connections: number;
    maxConnections: number;
    errorMessage?: string; // Error details if creation failed
    // Backup scheduling fields
    backupEnabled?: boolean;
    backupSchedule?: string; // cron expression
    backupRetentionCount?: number;
    lastBackupAt?: string;
}

export interface Backup {
    id: string;
    databaseId: string;
    databaseName: string;
    createdAt: string;
    size: number;
    status: 'completed' | 'in-progress' | 'failed';
}

export interface DatabaseMetrics {
    cpuPercent: number;
    memoryUsage: number;
    memoryLimit: number;
    memoryPercent: number;
    networkRx: number;
    networkTx: number;
    storageUsed: number;
    connections: number;
}

export interface DatabaseCredentials {
    username: string;
    password: string;
    database: string;
    host: string;
    port: number;
    engine: string;
}

export interface ConnectionExample {
    title: string;
    language: string;
    code: string;
    description: string;
}

export interface MetricsPoint {
    timestamp: string;
    cpuPercent: number;
    memoryUsage: number;
    memoryLimit: number;
    memoryPercent: number;
    storageUsed: number;
    connections: number;
    networkRx: number;
    networkTx: number;
}

export interface TopologyNode {
    id: string;
    name: string;
    engine: string;
    status: string;
    network: string;
}

export interface TopologyNetwork {
    name: string;
    databases: TopologyNode[];
}

export interface BackupInfo {
    id: string;
    databaseId: string;
    databaseName: string;
    createdAt: string;
    size: number;
    status: string;
    engine: string;
    version: string;
}

export interface MetricsHistoryPoint {
    timestamp: string;
    cpuPercent: number;
    memoryUsage: number;
    memoryLimit: number;
    memoryPercent: number;
    storageUsed: number;
    connections: number;
    networkRx: number;
    networkTx: number;
}

export interface CreateDatabaseRequest {
    name: string;
    engine: 'postgresql' | 'mysql' | 'mariadb' | 'redis';
    version: string;
    username: string;
    password?: string; // Optional - auto-generated if not provided
    database: string;
    storageLimit: number;
    memoryLimit: number;
    network?: string; // Docker network name
    exposePort?: boolean; // Whether to bind port to host
    // Restore from backup
    restoreFromBackupId?: string;
    // Backup settings
    backupEnabled?: boolean;
    backupSchedule?: string;
    backupRetentionCount?: number;
    // Data seeding
    seedSource?: 'none' | 'text' | 'file';
    seedContent?: string;
}


// Docker network info
export interface DockerNetwork {
    id: string;
    name: string;
    driver: string;
}

// Auth types
export interface User {
    id: string;
    username: string;
    createdAt: string;
}

export interface AuthStatus {
    enabled: boolean;
    configured: boolean;
}

export interface LoginRequest {
    username: string;
    password: string;
}

export interface RegisterRequest {
    username: string;
    password: string;
}

export interface LoginResponse extends User {
    token: string;
}

export interface ApiError {
    error: string;
}

class ApiClient {
    private async request<T>(
        endpoint: string,
        options: RequestInit = {}
    ): Promise<T> {
        const response = await fetch(`${API_BASE}${endpoint}`, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers,
            },
        });

        if (!response.ok) {
            const error: ApiError = await response.json().catch(() => ({ error: 'Unknown error' }));
            throw new Error(error.error || `HTTP ${response.status}`);
        }

        // Handle 204 No Content
        if (response.status === 204) {
            return {} as T;
        }

        return response.json();
    }

    // Health
    async health(): Promise<{ status: string; version: string }> {
        return this.request('/health');
    }

    // Databases
    async listDatabases(): Promise<DatabaseInstance[]> {
        const result = await this.request<DatabaseInstance[] | null>('/databases');
        return result || [];
    }

    async getDatabase(id: string): Promise<DatabaseInstance> {
        return this.request(`/databases/${id}`);
    }

    async createDatabase(data: CreateDatabaseRequest): Promise<DatabaseInstance> {
        return this.request('/databases', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async deleteDatabase(id: string): Promise<void> {
        await this.request(`/databases/${id}`, { method: 'DELETE' });
    }

    async startDatabase(id: string): Promise<DatabaseInstance> {
        return this.request(`/databases/${id}/start`, { method: 'POST' });
    }

    async stopDatabase(id: string): Promise<DatabaseInstance> {
        return this.request(`/databases/${id}/stop`, { method: 'POST' });
    }

    async getMetrics(databaseId: string): Promise<DatabaseMetrics> {
        return this.request(`/databases/${databaseId}/metrics`);
    }

    async restoreBackup(databaseId: string, backupId: string): Promise<void> {
        await this.request(`/databases/${databaseId}/restore`, {
            method: 'POST',
            body: JSON.stringify({ backupId }),
        });
    }

    async updateBackupSettings(id: string, settings: { backupEnabled: boolean; backupSchedule: string; backupRetentionCount: number }): Promise<DatabaseInstance> {
        return this.request(`/databases/${id}/backup-settings`, {
            method: 'PUT',
            body: JSON.stringify(settings),
        });
    }

    async updateResources(id: string, memoryLimit: number, cpuLimit: number): Promise<DatabaseInstance> {
        return this.request(`/databases/${id}/resources`, {
            method: 'PATCH',
            body: JSON.stringify({ memoryLimit: memoryLimit * 1024 * 1024, cpuLimit }), // Convert MB to bytes
        });
    }

    async getCredentials(id: string): Promise<DatabaseCredentials> {
        return this.request(`/databases/${id}/credentials`);
    }

    async getConnectionExamples(id: string): Promise<ConnectionExample[]> {
        return this.request(`/databases/${id}/connection-strings`);
    }

    async getBackupInfo(backupId: string): Promise<BackupInfo> {
        return this.request(`/backups/${backupId}/info`);
    }

    async getMetricsHistory(id: string): Promise<MetricsPoint[]> {
        return this.request(`/databases/${id}/metrics/history`);
    }

    async getTopology(): Promise<TopologyNetwork[]> {
        return this.request('/topology');
    }

    async getHealthCheck(databaseId: string): Promise<{
        status: string;
        healthy: boolean;
        containerId?: string;
        engine: string;
        host: string;
        port: number;
        connectionVerified?: boolean;
        connectionError?: string;
    }> {
        return this.request(`/databases/${databaseId}/health`);
    }

    async getLogs(id: string): Promise<{ logs: string }> {
        return this.request(`/databases/${id}/logs`);
    }

    async downloadBackup(backupId: string): Promise<void> {
        window.open(`${API_BASE}/backups/${backupId}/download`, '_blank');
    }

    // Backups
    async listBackups(databaseId?: string): Promise<Backup[]> {
        const query = databaseId ? `?databaseId=${databaseId}` : '';
        const result = await this.request<Backup[] | null>(`/backups${query}`);
        return result || [];
    }

    async createBackup(databaseId: string): Promise<Backup> {
        return this.request(`/databases/${databaseId}/backup`, { method: 'POST' });
    }

    // Networks
    async listNetworks(): Promise<DockerNetwork[]> {
        const result = await this.request<DockerNetwork[] | null>('/networks');
        return result || [];
    }

    async createNetwork(name: string): Promise<DockerNetwork> {
        return this.request('/networks', {
            method: 'POST',
            body: JSON.stringify({ name }),
        });
    }

    async deleteNetwork(name: string): Promise<void> {
        await this.request(`/networks/${name}`, { method: 'DELETE' });
    }

    // Auth
    async authStatus(): Promise<AuthStatus> {
        return this.request('/auth/status');
    }

    async register(data: RegisterRequest): Promise<User> {
        return this.request('/auth/register', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async login(data: LoginRequest): Promise<LoginResponse> {
        return this.request('/auth/login', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async logout(): Promise<void> {
        await this.request('/auth/logout', { method: 'POST' });
    }

    async getCurrentUser(): Promise<User> {
        return this.request('/auth/me');
    }



    async bulkStart(ids: string[]): Promise<{ message: string; errors?: string[] }> {
        return this.request('/databases/bulk/start', {
            method: 'POST',
            body: JSON.stringify({ ids }),
        });
    }

    async bulkStop(ids: string[]): Promise<{ message: string; errors?: string[] }> {
        return this.request('/databases/bulk/stop', {
            method: 'POST',
            body: JSON.stringify({ ids }),
        });
    }

    async bulkDelete(ids: string[]): Promise<{ message: string; errors?: string[] }> {
        return this.request('/databases/bulk/delete', {
            method: 'POST',
            body: JSON.stringify({ ids }),
        });
    }

    async deleteBackup(id: string): Promise<void> {
        await this.request(`/backups/${id}`, { method: 'DELETE' });
    }
}

export const api = new ApiClient();
