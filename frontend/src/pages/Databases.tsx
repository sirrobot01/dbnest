"use client";

import { useState, useEffect, useCallback } from "react";
import { Sidebar } from "@/components/Sidebar";
import { DatabaseCard } from "@/components/DatabaseCard";
import { CreateDatabaseModal } from "@/components/CreateDatabaseModal";
import { Input } from "@/components/ui/input";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { api, DatabaseInstance } from "@/lib/api";
import { Search, Loader2, Database, AlertCircle } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";

const engineConfig = {
    postgresql: { name: "PostgreSQL" },
    mysql: { name: "MySQL" },
    redis: { name: "Redis" },
    mariadb: { name: "MariaDB" },
};

export default function Databases() {
    const [createModalOpen, setCreateModalOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");
    const [engineFilter, setEngineFilter] = useState<string>("all");
    const [statusFilter, setStatusFilter] = useState<string>("all");
    const [databases, setDatabases] = useState<DatabaseInstance[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const fetchDatabases = useCallback(async () => {
        try {
            setError(null);
            const data = await api.listDatabases();
            setDatabases(data);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch databases');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchDatabases();

        // Refresh when page becomes visible (focus-based refresh)
        const handleVisibilityChange = () => {
            if (document.visibilityState === 'visible') {
                fetchDatabases();
            }
        };

        document.addEventListener('visibilitychange', handleVisibilityChange);
        return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
    }, [fetchDatabases]);

    const filteredDatabases = databases.filter((db) => {
        const matchesSearch =
            db.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
            db.database.toLowerCase().includes(searchQuery.toLowerCase());
        const matchesEngine = engineFilter === "all" || db.engine === engineFilter;
        const matchesStatus = statusFilter === "all" || db.status === statusFilter;
        return matchesSearch && matchesEngine && matchesStatus;
    });

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

    return (
        <div className="flex min-h-screen bg-background">
            <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />

            <main className="flex-1 overflow-auto">
                <header className="sticky top-0 z-10 bg-background/80 backdrop-blur-sm border-b border-border">
                    <div className="px-6 py-4">
                        <h1 className="text-2xl font-bold">Databases</h1>
                        <p className="text-muted-foreground text-sm">
                            Manage all your database instances
                        </p>
                    </div>
                </header>

                <div className="p-6 space-y-6">
                    {error && (
                        <Alert variant="destructive">
                            <AlertCircle className="h-4 w-4" />
                            <AlertDescription>{error}</AlertDescription>
                        </Alert>
                    )}

                    <div className="flex flex-col sm:flex-row gap-4">
                        <div className="relative flex-1">
                            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                            <Input
                                placeholder="Search databases..."
                                className="pl-10"
                                value={searchQuery}
                                onChange={(e) => setSearchQuery(e.target.value)}
                            />
                        </div>
                        <Select value={engineFilter} onValueChange={setEngineFilter}>
                            <SelectTrigger className="w-[180px]">
                                <SelectValue placeholder="All Engines" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">All Engines</SelectItem>
                                {Object.entries(engineConfig).map(([key, config]) => (
                                    <SelectItem key={key} value={key}>
                                        {config.name}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        <Select value={statusFilter} onValueChange={setStatusFilter}>
                            <SelectTrigger className="w-[180px]">
                                <SelectValue placeholder="All Status" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">All Status</SelectItem>
                                <SelectItem value="running">Running</SelectItem>
                                <SelectItem value="stopped">Stopped</SelectItem>
                                <SelectItem value="error">Error</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    {loading ? (
                        <div className="flex items-center justify-center py-12">
                            <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
                        </div>
                    ) : filteredDatabases.length > 0 ? (
                        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                            {filteredDatabases.map((database) => (
                                <DatabaseCard
                                    key={database.id}
                                    database={database}
                                    onStart={handleStartDatabase}
                                    onStop={handleStopDatabase}
                                    onDelete={handleDeleteDatabase}
                                />
                            ))}
                        </div>
                    ) : (
                        <div className="text-center py-12 text-muted-foreground">
                            <Database className="w-12 h-12 mx-auto mb-4 opacity-50" />
                            <p>
                                {databases.length === 0
                                    ? "No databases yet"
                                    : "No databases match your filters"}
                            </p>
                        </div>
                    )}

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
