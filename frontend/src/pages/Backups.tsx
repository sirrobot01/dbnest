"use client";

import { useState, useEffect, useCallback } from "react";
import { Sidebar } from "@/components/Sidebar";
import { CreateDatabaseModal } from "@/components/CreateDatabaseModal";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { api, Backup } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Download, RotateCcw, Loader2, Archive, AlertCircle } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";

export default function BackupsPage() {
    const [createModalOpen, setCreateModalOpen] = useState(false);
    const [backups, setBackups] = useState<Backup[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const fetchBackups = useCallback(async () => {
        try {
            setError(null);
            const data = await api.listBackups();
            setBackups(data);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch backups');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchBackups();
    }, [fetchBackups]);

    const formatDate = (date: string) => {
        return new Date(date).toLocaleDateString("en-US", {
            year: "numeric",
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
        });
    };

    const formatSize = (bytes: number) => {
        const mb = bytes / (1024 * 1024);
        if (mb >= 1024) {
            return `${(mb / 1024).toFixed(1)} GB`;
        }
        return `${mb.toFixed(1)} MB`;
    };

    return (
        <div className="flex min-h-screen bg-background">
            <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />

            <main className="flex-1 overflow-auto">
                <header className="sticky top-0 z-10 bg-background/80 backdrop-blur-sm border-b border-border">
                    <div className="px-6 py-4">
                        <h1 className="text-2xl font-bold">Backups</h1>
                        <p className="text-muted-foreground text-sm">
                            Manage your database backups
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

                    {loading ? (
                        <div className="flex items-center justify-center py-12">
                            <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
                        </div>
                    ) : backups.length > 0 ? (
                        <Card>
                            <CardHeader>
                                <CardTitle className="text-base">Backup History</CardTitle>
                            </CardHeader>
                            <CardContent className="p-0">
                                <div className="divide-y divide-border">
                                    {backups.map((backup) => (
                                        <div
                                            key={backup.id}
                                            className="flex items-center justify-between p-4"
                                        >
                                            <div className="flex items-center gap-4">
                                                <div className="p-2 rounded-lg bg-muted">
                                                    <Download className="w-4 h-4" />
                                                </div>
                                                <div>
                                                    <p className="font-medium">{backup.databaseName}</p>
                                                    <p className="text-sm text-muted-foreground">
                                                        {formatDate(backup.createdAt)} • {formatSize(backup.size)}
                                                    </p>
                                                </div>
                                            </div>
                                            <div className="flex items-center gap-2">
                                                <Badge
                                                    variant="outline"
                                                    className={cn(
                                                        backup.status === "completed" &&
                                                        "bg-emerald-500/10 text-emerald-500 border-0",
                                                        backup.status === "in-progress" &&
                                                        "bg-amber-500/10 text-amber-500 border-0",
                                                        backup.status === "failed" &&
                                                        "bg-red-500/10 text-red-500 border-0"
                                                    )}
                                                >
                                                    {backup.status}
                                                </Badge>
                                                <Button variant="outline" size="sm">
                                                    <Download className="w-4 h-4 mr-2" />
                                                    Download
                                                </Button>
                                                <Button variant="outline" size="sm">
                                                    <RotateCcw className="w-4 h-4 mr-2" />
                                                    Restore
                                                </Button>
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            </CardContent>
                        </Card>
                    ) : (
                        <div className="text-center py-12 text-muted-foreground">
                            <Archive className="w-12 h-12 mx-auto mb-4 opacity-50" />
                            <p>No backups yet</p>
                            <p className="text-sm mt-1">
                                Create a backup from any database detail page
                            </p>
                        </div>
                    )}
                </div>
            </main>

            <CreateDatabaseModal
                open={createModalOpen}
                onOpenChange={setCreateModalOpen}
                onCreate={fetchBackups}
            />
        </div>
    );
}
