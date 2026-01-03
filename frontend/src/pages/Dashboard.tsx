"use client";

import { useState, useEffect, useCallback } from "react";
import { Sidebar } from "@/components/Sidebar";
import { DatabaseCard } from "@/components/DatabaseCard";
import { StatsCard } from "@/components/StatsCard";
import { CreateDatabaseModal } from "@/components/CreateDatabaseModal";
import { api, DatabaseInstance } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Database, Activity, HardDrive, Zap, Loader2, AlertCircle, Play, Square, Trash2, X, RefreshCw } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { toast } from "sonner";

export default function Dashboard() {
    const [databases, setDatabases] = useState<DatabaseInstance[]>([]);
    const [loading, setLoading] = useState(true);
    const [isRefreshing, setIsRefreshing] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [createModalOpen, setCreateModalOpen] = useState(false);
    const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
    const [bulkLoading, setBulkLoading] = useState<string | null>(null);

    const fetchDatabases = useCallback(async () => {
        try {
            setError(null);
            const data = await api.listDatabases();
            setDatabases(data);
            // Clean up any selected IDs that no longer exist
            setSelectedIds(prev => {
                const existingIds = new Set(data.map(d => d.id));
                const newSelected = new Set([...prev].filter(id => existingIds.has(id)));
                return newSelected.size !== prev.size ? newSelected : prev;
            });
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch databases');
        } finally {
            setLoading(false);
            setIsRefreshing(false);
        }
    }, []);

    useEffect(() => {
        fetchDatabases();

        // Refresh when page becomes visible
        const handleVisibilityChange = () => {
            if (document.visibilityState === 'visible') {
                fetchDatabases();
            }
        };
        document.addEventListener('visibilitychange', handleVisibilityChange);
        return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
    }, [fetchDatabases]);

    const handleRefresh = async () => {
        setIsRefreshing(true);
        await fetchDatabases();
    };

    const runningDbs = databases.filter((db) => db.status === "running").length;
    const totalStorage = databases.reduce((acc, db) => acc + (db.storageUsed || 0), 0);
    const totalConnections = databases.reduce((acc, db) => acc + (db.connections || 0), 0);

    const handleToggleSelect = (id: string) => {
        setSelectedIds(prev => {
            const newSet = new Set(prev);
            if (newSet.has(id)) {
                newSet.delete(id);
            } else {
                newSet.add(id);
            }
            return newSet;
        });
    };

    const handleSelectAll = () => {
        if (selectedIds.size === databases.length) {
            setSelectedIds(new Set());
        } else {
            setSelectedIds(new Set(databases.map(d => d.id)));
        }
    };

    const handleBulkStart = async () => {
        setBulkLoading('start');
        try {
            await api.bulkStart([...selectedIds]);
            toast.success(`Started ${selectedIds.size} databases`);
            setSelectedIds(new Set());
            fetchDatabases();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to start databases');
        } finally {
            setBulkLoading(null);
        }
    };

    const handleBulkStop = async () => {
        setBulkLoading('stop');
        try {
            await api.bulkStop([...selectedIds]);
            toast.success(`Stopped ${selectedIds.size} databases`);
            setSelectedIds(new Set());
            fetchDatabases();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to stop databases');
        } finally {
            setBulkLoading(null);
        }
    };

    const handleBulkDelete = async () => {
        if (!confirm(`Are you sure you want to delete ${selectedIds.size} databases? This cannot be undone.`)) return;
        setBulkLoading('delete');
        try {
            await api.bulkDelete([...selectedIds]);
            toast.success(`Deleted ${selectedIds.size} databases`);
            setSelectedIds(new Set());
            fetchDatabases();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to delete databases');
        } finally {
            setBulkLoading(null);
        }
    };

    const handleStartDatabase = async (id: string) => {
        try {
            await api.startDatabase(id);
            await fetchDatabases();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to start database');
        }
    };

    const handleStopDatabase = async (id: string) => {
        try {
            await api.stopDatabase(id);
            await fetchDatabases();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to stop database');
        }
    };

    const handleDeleteDatabase = async (id: string) => {
        try {
            await api.deleteDatabase(id);
            await fetchDatabases();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to delete database');
        }
    };

    const formatStorage = (bytes: number) => {
        const mb = bytes / (1024 * 1024);
        if (mb >= 1024) {
            return `${(mb / 1024).toFixed(1)} GB`;
        }
        return `${mb.toFixed(0)} MB`;
    };

    return (
        <div className="flex min-h-screen bg-background">
            <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />

            <main className="flex-1 overflow-auto">
                <header className="sticky top-0 z-10 bg-background/80 backdrop-blur-sm border-b border-border">
                    <div className="px-6 py-4 flex items-center justify-between">
                        <div>
                            <h1 className="text-2xl font-bold">Dashboard</h1>
                            <p className="text-muted-foreground text-sm">
                                Manage your database instances
                            </p>
                        </div>
                        <Button variant="outline" size="sm" onClick={handleRefresh} disabled={isRefreshing}>
                            <RefreshCw className={`w-4 h-4 mr-2 ${isRefreshing ? 'animate-spin' : ''}`} />
                            Refresh
                        </Button>
                    </div>
                </header>

                <div className="p-6 space-y-6">
                    {error && (
                        <Alert variant="destructive">
                            <AlertCircle className="h-4 w-4" />
                            <AlertDescription>{error}</AlertDescription>
                        </Alert>
                    )}

                    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                        <StatsCard
                            title="Total Databases"
                            value={databases.length}
                            description={`${runningDbs} currently running`}
                            icon={Database}
                        />
                        <StatsCard
                            title="Active Connections"
                            value={totalConnections}
                            description="Across all databases"
                            icon={Activity}
                        />
                        <StatsCard
                            title="Storage Used"
                            value={formatStorage(totalStorage)}
                            description="Total across all instances"
                            icon={HardDrive}
                        />
                        <StatsCard
                            title="System Health"
                            value={error ? "Error" : "Healthy"}
                            description={error ? "Check logs" : "All systems operational"}
                            icon={Zap}
                        />
                    </div>

                    <div>
                        <div className="flex items-center justify-between mb-4">
                            <h2 className="text-lg font-semibold">Your Databases</h2>
                            {databases.length > 0 && (
                                <Button variant="outline" size="sm" onClick={handleSelectAll}>
                                    {selectedIds.size === databases.length ? 'Deselect All' : 'Select All'}
                                </Button>
                            )}
                        </div>

                        {/* Bulk Action Bar */}
                        {selectedIds.size > 0 && (
                            <div className="mb-4 p-3 bg-muted rounded-lg border flex items-center justify-between">
                                <span className="text-sm font-medium">
                                    {selectedIds.size} database{selectedIds.size > 1 ? 's' : ''} selected
                                </span>
                                <div className="flex gap-2">
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={handleBulkStart}
                                        disabled={bulkLoading !== null}
                                    >
                                        {bulkLoading === 'start' ? (
                                            <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                                        ) : (
                                            <Play className="w-4 h-4 mr-1" />
                                        )}
                                        Start All
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={handleBulkStop}
                                        disabled={bulkLoading !== null}
                                    >
                                        {bulkLoading === 'stop' ? (
                                            <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                                        ) : (
                                            <Square className="w-4 h-4 mr-1" />
                                        )}
                                        Stop All
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        className="text-destructive hover:text-destructive"
                                        onClick={handleBulkDelete}
                                        disabled={bulkLoading !== null}
                                    >
                                        {bulkLoading === 'delete' ? (
                                            <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                                        ) : (
                                            <Trash2 className="w-4 h-4 mr-1" />
                                        )}
                                        Delete All
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="ghost"
                                        onClick={() => setSelectedIds(new Set())}
                                    >
                                        <X className="w-4 h-4" />
                                    </Button>
                                </div>
                            </div>
                        )}

                        {loading ? (
                            <div className="flex items-center justify-center py-12">
                                <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
                            </div>
                        ) : databases.length === 0 ? (
                            <div className="text-center py-12 text-muted-foreground">
                                <Database className="w-12 h-12 mx-auto mb-4 opacity-50" />
                                <p>No databases yet. Create your first one!</p>
                            </div>
                        ) : (
                            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                                {databases.map((database) => (
                                    <DatabaseCard
                                        key={database.id}
                                        database={database}
                                        onStart={handleStartDatabase}
                                        onStop={handleStopDatabase}
                                        onDelete={handleDeleteDatabase}
                                        selected={selectedIds.has(database.id)}
                                        onToggleSelect={handleToggleSelect}
                                    />
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            </main>

            <CreateDatabaseModal
                open={createModalOpen}
                onOpenChange={setCreateModalOpen}
                onCreate={fetchDatabases}
            />
        </div>
    );
}
