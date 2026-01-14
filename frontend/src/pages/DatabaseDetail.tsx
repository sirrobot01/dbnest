"use client";

import { useState, useEffect, useCallback } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { Sidebar } from "@/components/Sidebar";
import { CreateDatabaseModal } from "@/components/CreateDatabaseModal";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Switch } from "@/components/ui/switch";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { api, DatabaseInstance, Backup, DatabaseMetrics, DatabaseCredentials, ConnectionExample, BackupInfo, MetricsPoint } from "@/lib/api";
import { MetricsCharts } from "@/components/MetricsCharts";
import { toast } from "sonner";
import {
    ArrowLeft,
    Copy,
    Play,
    Square,
    Trash2,
    Download,
    Database,
    HardDrive,
    Cpu,
    MemoryStick,
    Users,
    Clock,
    Check,
    Loader2,
    AlertCircle,
    RotateCcw,
    Eye,
    EyeOff,
    RefreshCw,
    Link2,
    Terminal,
} from "lucide-react";
import { formatBytes } from "@/lib/utils";

const engineConfig: Record<string, { name: string }> = {
    postgresql: { name: "PostgreSQL" },
    mysql: { name: "MySQL" },
    mariadb: { name: "MariaDB" },
    redis: { name: "Redis" },
};

export default function DatabaseDetail() {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();

    const [createModalOpen, setCreateModalOpen] = useState(false);
    const [copied, setCopied] = useState<string | null>(null);
    const [database, setDatabase] = useState<DatabaseInstance | null>(null);
    const [backups, setBackups] = useState<Backup[]>([]);
    const [metrics, setMetrics] = useState<DatabaseMetrics | null>(null);
    const [loading, setLoading] = useState(true);
    const [isRefreshing, setIsRefreshing] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [actionLoading, setActionLoading] = useState<string | null>(null);

    // Backup settings state
    const [backupEnabled, setBackupEnabled] = useState(false);
    const [backupSchedule, setBackupSchedule] = useState("0 0 * * *");
    const [backupRetention, setBackupRetention] = useState(7);
    const [savingBackupSettings, setSavingBackupSettings] = useState(false);

    // Resource settings state
    const [memoryLimit, setMemoryLimit] = useState(256);
    const [cpuLimit, setCpuLimit] = useState(0.5);
    const [savingResources, setSavingResources] = useState(false);

    // Credentials and connection examples state
    const [credentials, setCredentials] = useState<DatabaseCredentials | null>(null);
    const [connectionExamples, setConnectionExamples] = useState<ConnectionExample[]>([]);
    const [selectedExampleTab, setSelectedExampleTab] = useState(0);
    const [passwordVisible, setPasswordVisible] = useState(false);
    const [loadingCredentials, setLoadingCredentials] = useState(false);

    // Logs state
    const [dbLogs, setDbLogs] = useState<string>("");
    const [loadingLogs, setLoadingLogs] = useState(false);

    // Metrics history state
    const [metricsHistory, setMetricsHistory] = useState<MetricsPoint[]>([]);
    const [loadingMetrics, setLoadingMetrics] = useState(true);

    // Backup preview state
    const [previewBackup, setPreviewBackup] = useState<BackupInfo | null>(null);
    const [loadingPreview, setLoadingPreview] = useState(false);

    const fetchData = useCallback(async () => {
        if (!id) return;

        try {
            setError(null);
            const [db, bks] = await Promise.all([
                api.getDatabase(id),
                api.listBackups(id),
            ]);
            setDatabase(db);
            setBackups(bks);

            // Set backup settings from database
            setBackupEnabled(db.backupEnabled || false);
            setBackupSchedule(db.backupSchedule || "0 0 * * *");
            setBackupRetention(db.backupRetentionCount || 7);

            // Set resource settings from database
            setMemoryLimit(db.memoryLimit || 256);
            setCpuLimit(db.cpuLimit || 0.5);

            if (db.status === "running") {
                const metricsData = await api.getMetrics(id).catch(() => null);
                setMetrics(metricsData);
            } else {
                setMetrics(null);
            }
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch database');
        } finally {
            setLoading(false);
            setIsRefreshing(false);
        }
    }, [id]);

    const fetchMetrics = useCallback(async () => {
        if (!id) return;
        try {
            const history = await api.getMetricsHistory(id);
            setMetricsHistory(history);
        } catch (err) {
            console.error('Failed to fetch metrics:', err);
        } finally {
            setLoadingMetrics(false);
        }
    }, [id]);

    useEffect(() => {
        if (!id) return;
        fetchData();
        fetchMetrics();

        const handleVisibilityChange = () => {
            if (document.visibilityState === 'visible') {
                fetchData();
                fetchMetrics();
            }
        };
        document.addEventListener('visibilitychange', handleVisibilityChange);

        const metricsInterval = setInterval(fetchMetrics, 30000); // Refresh metrics every 30s
        return () => {
            clearInterval(metricsInterval);
            document.removeEventListener('visibilitychange', handleVisibilityChange);
        };
    }, [id, fetchData, fetchMetrics]);

    const handleRefresh = async () => {
        setIsRefreshing(true);
        await Promise.all([fetchData(), fetchMetrics()]);
    };

    // ... (rest of logic up to header)

    const copyToClipboard = async (text: string, label: string) => {
        try {
            await navigator.clipboard.writeText(text);
            setCopied(label);
            setTimeout(() => setCopied(null), 2000);
            toast.success(`${label} copied`);
        } catch (err) {
            console.error('Copy failed:', err);
        }
    };

    const handleStart = async () => {
        if (!id) return;
        setActionLoading('start');
        try {
            await api.startDatabase(id);
            toast.success('Database started');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to start database');
        } finally {
            setActionLoading(null);
        }
    };

    const handleStop = async () => {
        if (!id) return;
        setActionLoading('stop');
        try {
            await api.stopDatabase(id);
            toast.success('Database stopped');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to stop database');
        } finally {
            setActionLoading(null);
        }
    };

    const handleBackup = async () => {
        if (!id) return;
        setActionLoading('backup');
        try {
            await api.createBackup(id);
            toast.success('Backup created');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to create backup');
        } finally {
            setActionLoading(null);
        }
    };

    const handleDelete = async () => {
        if (!id) return;
        if (!confirm('Are you sure you want to delete this database? This cannot be undone.')) return;
        setActionLoading('delete');
        try {
            await api.deleteDatabase(id);
            toast.success('Database deleted');
            navigate('/');
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to delete database');
        } finally {
            setActionLoading(null);
        }
    };

    const handleDeleteBackup = async (backupId: string) => {
        if (!confirm('Are you sure you want to delete this backup?')) return;
        setActionLoading(`delete-backup-${backupId}`);
        try {
            await api.deleteBackup(backupId);
            toast.success('Backup deleted');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to delete backup');
        } finally {
            setActionLoading(null);
        }
    };



    const handleSaveBackupSettings = async () => {
        if (!id) return;
        setSavingBackupSettings(true);
        try {
            await api.updateBackupSettings(id, {
                backupEnabled,
                backupSchedule,
                backupRetentionCount: backupRetention,
            });
            toast.success('Backup settings saved');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to save backup settings');
        } finally {
            setSavingBackupSettings(false);
        }
    };

    const handleSaveResources = async () => {
        if (!id) return;
        setSavingResources(true);
        try {
            await api.updateResources(id, memoryLimit, cpuLimit);
            toast.success('Resource limits updated');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to update resources');
        } finally {
            setSavingResources(false);
        }
    };

    const fetchCredentials = async () => {
        if (!id || credentials) return;
        setLoadingCredentials(true);
        try {
            const [creds, examples] = await Promise.all([
                api.getCredentials(id),
                api.getConnectionExamples(id),
            ]);
            setCredentials(creds);
            setConnectionExamples(examples);
        } catch (err) {
            toast.error('Failed to fetch credentials');
        } finally {
            setLoadingCredentials(false);
        }
    };

    const fetchLogs = async () => {
        if (!id) return;
        setLoadingLogs(true);
        try {
            const data = await api.getLogs(id);
            setDbLogs(data.logs);
        } catch (err) {
            toast.error('Failed to fetch logs: ' + (err instanceof Error ? err.message : String(err)));
        } finally {
            setLoadingLogs(false);
        }
    };


    const handlePreviewBackup = async (backupId: string) => {
        setLoadingPreview(true);
        try {
            const info = await api.getBackupInfo(backupId);
            setPreviewBackup(info);
        } catch (err) {
            toast.error('Failed to load backup info');
        } finally {
            setLoadingPreview(false);
        }
    };

    const handleConfirmRestore = async () => {
        if (!id || !previewBackup) return;
        setPreviewBackup(null);
        setActionLoading('restore');
        try {
            await api.restoreBackup(id, previewBackup.id);
            toast.success('Backup restoration started');
            fetchData();
        } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to restore backup');
        } finally {
            setActionLoading(null);
        }
    };




    const formatDate = (date: string) => {
        return new Date(date).toLocaleDateString("en-US", {
            year: "numeric",
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
        });
    };

    if (loading) {
        return (
            <div className="flex min-h-screen">
                <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />
                <main className="flex-1 p-8 overflow-auto">
                    <div className="flex items-center justify-center h-64">
                        <Loader2 className="w-8 h-8 animate-spin text-primary" />
                    </div>
                </main>
                <CreateDatabaseModal open={createModalOpen} onOpenChange={setCreateModalOpen} onCreate={fetchData} />
            </div>
        );
    }

    if (error || !database) {
        return (
            <div className="flex min-h-screen">
                <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />
                <main className="flex-1 p-8 overflow-auto">
                    <Alert variant="destructive">
                        <AlertCircle className="h-4 w-4" />
                        <AlertDescription>{error || 'Database not found'}</AlertDescription>
                    </Alert>
                    <Button asChild className="mt-4">
                        <Link to="/">
                            <ArrowLeft className="w-4 h-4 mr-2" />
                            Back to Databases
                        </Link>
                    </Button>
                </main>
                <CreateDatabaseModal open={createModalOpen} onOpenChange={setCreateModalOpen} onCreate={fetchData} />
            </div>
        );
    }

    return (
        <div className="flex min-h-screen">
            <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />
            <main className="flex-1 overflow-auto">
                <header className="sticky top-0 z-10 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 border-b">
                    <div className="flex items-center justify-between p-4">
                        <div className="flex items-center gap-4">
                            <Button variant="ghost" size="icon" asChild>
                                <Link to="/">
                                    <ArrowLeft className="h-4 w-4" />
                                </Link>
                            </Button>
                            <div>
                                <div className="flex items-center gap-3">
                                    <h1 className="text-xl font-semibold">{database.name}</h1>
                                    <StatusBadge status={database.status} />
                                </div>
                                <p className="text-sm text-muted-foreground">
                                    {engineConfig[database.engine]?.name || database.engine} {database.version}
                                </p>
                            </div>
                        </div>
                        <div className="flex items-center gap-2">
                            <Button variant="outline" size="sm" onClick={handleRefresh} disabled={isRefreshing}>
                                <RefreshCw className={`w-4 h-4 mr-2 ${isRefreshing ? 'animate-spin' : ''}`} />
                                Refresh
                            </Button>
                            {database.status === "stopped" ? (
                                <Button
                                    size="sm"
                                    className="bg-emerald-600 hover:bg-emerald-700 text-white border-emerald-600"
                                    onClick={handleStart}
                                    disabled={actionLoading === 'start'}
                                >
                                    {actionLoading === 'start' ? (
                                        <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    ) : (
                                        <Play className="w-4 h-4 mr-2" />
                                    )}
                                    Start
                                </Button>
                            ) : database.status === "running" ? (
                                <Button
                                    size="sm"
                                    variant="outline"
                                    className="border-amber-500/50 text-amber-600 hover:bg-amber-500/10 hover:text-amber-700 hover:border-amber-500"
                                    onClick={handleStop}
                                    disabled={actionLoading === 'stop'}
                                >
                                    {actionLoading === 'stop' ? (
                                        <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    ) : (
                                        <Square className="w-4 h-4 mr-2 fill-current" />
                                    )}
                                    Stop
                                </Button>
                            ) : null}
                            <Button size="sm" variant="outline" onClick={handleBackup} disabled={actionLoading === 'backup'}>
                                {actionLoading === 'backup' ? (
                                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                ) : (
                                    <Download className="w-4 h-4 mr-2" />
                                )}
                                Backup
                            </Button>
                            <Button
                                size="sm"
                                variant="outline"
                                className="text-destructive hover:text-destructive"
                                onClick={handleDelete}
                                disabled={actionLoading === 'delete'}
                            >
                                {actionLoading === 'delete' ? (
                                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                ) : (
                                    <Trash2 className="w-4 h-4 mr-2" />
                                )}
                                Delete
                            </Button>
                        </div>
                    </div>
                </header>

                <div className="p-6">
                    <Tabs defaultValue="overview" className="space-y-6">
                        <TabsList>
                            <TabsTrigger value="overview">Overview</TabsTrigger>
                            <TabsTrigger value="metrics">Metrics</TabsTrigger>
                            <TabsTrigger value="logs">Logs</TabsTrigger>
                            <TabsTrigger value="connection">Connection</TabsTrigger>
                            <TabsTrigger value="backups">Backups</TabsTrigger>
                            <TabsTrigger value="settings">Settings</TabsTrigger>
                        </TabsList>

                        {/* Logs Tab */}
                        <TabsContent value="logs" className="space-y-6" onFocus={fetchLogs}>
                            <Card className="flex flex-col h-[600px]">
                                <CardHeader className="flex flex-row items-center justify-between pb-2 border-b">
                                    <div className="flex items-center gap-2">
                                        <Terminal className="w-4 h-4 mr-2" />
                                        <CardTitle className="text-base">System Logs</CardTitle>
                                    </div>
                                    <Button size="sm" variant="ghost" onClick={fetchLogs} disabled={loadingLogs}>
                                        <RefreshCw className={`w-4 h-4 mr-2 ${loadingLogs ? 'animate-spin' : ''}`} />
                                        Refresh
                                    </Button>
                                </CardHeader>
                                <CardContent className="flex-1 p-0 overflow-hidden bg-zinc-950 rounded-b-lg">
                                    {loadingLogs && !dbLogs ? (
                                        <div className="flex items-center justify-center h-full">
                                            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
                                        </div>
                                    ) : (
                                        <pre className="h-full p-4 overflow-auto text-xs font-mono text-zinc-300 leading-relaxed whitespace-pre-wrap">
                                            {dbLogs || "No logs available."}
                                        </pre>
                                    )}
                                </CardContent>
                            </Card>
                        </TabsContent>

                        {/* Overview Tab */}
                        <TabsContent value="overview" className="space-y-6">
                            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                                <Card>
                                    <CardHeader className="flex flex-row items-center justify-between pb-2">
                                        <CardTitle className="text-sm font-medium text-muted-foreground">Status</CardTitle>
                                        <Database className="w-4 h-4 text-muted-foreground" />
                                    </CardHeader>
                                    <CardContent>
                                        <StatusBadge status={database.status} />
                                    </CardContent>
                                </Card>
                                <Card>
                                    <CardHeader className="flex flex-row items-center justify-between pb-2">
                                        <CardTitle className="text-sm font-medium text-muted-foreground">Memory</CardTitle>
                                        <MemoryStick className="w-4 h-4 text-muted-foreground" />
                                    </CardHeader>
                                    <CardContent>
                                        <div className="text-2xl font-bold">
                                            {metrics ? formatBytes(metrics.memoryUsage) : '-'}
                                        </div>
                                        <p className="text-xs text-muted-foreground">
                                            of {formatBytes(database.memoryLimit)}
                                        </p>
                                    </CardContent>
                                </Card>
                                <Card>
                                    <CardHeader className="flex flex-row items-center justify-between pb-2">
                                        <CardTitle className="text-sm font-medium text-muted-foreground">CPU</CardTitle>
                                        <Cpu className="w-4 h-4 text-muted-foreground" />
                                    </CardHeader>
                                    <CardContent>
                                        <div className="text-2xl font-bold">
                                            {metrics ? `${metrics.cpuPercent.toFixed(1)}%` : '-'}
                                        </div>
                                        <p className="text-xs text-muted-foreground">utilization</p>
                                    </CardContent>
                                </Card>
                                <Card>
                                    <CardHeader className="flex flex-row items-center justify-between pb-2">
                                        <CardTitle className="text-sm font-medium text-muted-foreground">Storage</CardTitle>
                                        <HardDrive className="w-4 h-4 text-muted-foreground" />
                                    </CardHeader>
                                    <CardContent>
                                        <div className="text-2xl font-bold">
                                            {formatBytes(database.storageUsed)}
                                        </div>
                                        <p className="text-xs text-muted-foreground">used</p>
                                    </CardContent>
                                </Card>
                            </div>

                            {/* Connection Info */}
                            <Card>
                                <CardHeader>
                                    <CardTitle className="text-base">Connection Information</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Host</p>
                                            <div className="flex items-center gap-2">
                                                <code className="text-sm">{database.host}</code>
                                                <Button size="icon" variant="ghost" className="h-6 w-6" onClick={() => copyToClipboard(database.host, 'host')}>
                                                    {copied === 'host' ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                                                </Button>
                                            </div>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Port</p>
                                            <div className="flex items-center gap-2">
                                                <code className="text-sm">{database.port}</code>
                                                <Button size="icon" variant="ghost" className="h-6 w-6" onClick={() => copyToClipboard(String(database.port), 'port')}>
                                                    {copied === 'port' ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                                                </Button>
                                            </div>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Database</p>
                                            <code className="text-sm">{database.database}</code>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Username</p>
                                            <code className="text-sm">{database.username}</code>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>

                            {/* Details */}
                            <Card>
                                <CardHeader>
                                    <CardTitle className="text-base">Details</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <div className="grid gap-4 md:grid-cols-2">
                                        <div className="space-y-4">
                                            <div className="flex items-center gap-2">
                                                <Clock className="w-4 h-4 text-muted-foreground" />
                                                <div>
                                                    <p className="text-sm font-medium">Created</p>
                                                    <p className="text-sm text-muted-foreground">
                                                        {formatDate(database.createdAt)}
                                                    </p>
                                                </div>
                                            </div>
                                        </div>
                                        <div className="space-y-4">
                                            <div className="flex items-center gap-2">
                                                <Users className="w-4 h-4 text-muted-foreground" />
                                                <div>
                                                    <p className="text-sm font-medium">Connections</p>
                                                    <p className="text-sm text-muted-foreground">
                                                        {database.connections} / {database.maxConnections}
                                                    </p>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                        </TabsContent>

                        {/* Metrics Tab */}
                        <TabsContent value="metrics" className="space-y-6">
                            <div>
                                <h2 className="text-lg font-semibold">Performance Metrics</h2>
                                <p className="text-sm text-muted-foreground">
                                    Real-time performance monitoring. Data refreshes every 30 seconds.
                                </p>
                            </div>
                            <MetricsCharts data={metricsHistory} isLoading={loadingMetrics} />
                        </TabsContent>

                        {/* Connection Tab */}
                        <TabsContent value="connection" className="space-y-6" onFocus={fetchCredentials}>
                            <Card>
                                <CardHeader>
                                    <CardTitle className="text-base flex items-center gap-2">
                                        <Link2 className="w-4 h-4" />
                                        Database Credentials
                                    </CardTitle>
                                </CardHeader>
                                <CardContent className="space-y-4">
                                    {loadingCredentials ? (
                                        <div className="flex items-center justify-center py-8">
                                            <Loader2 className="w-6 h-6 animate-spin" />
                                        </div>
                                    ) : credentials ? (
                                        <div className="grid gap-4 md:grid-cols-2">
                                            <div className="space-y-2">
                                                <label className="text-sm font-medium text-muted-foreground">Host</label>
                                                <div className="flex items-center gap-2">
                                                    <code className="flex-1 px-3 py-2 bg-muted rounded text-sm">{credentials.host}</code>
                                                    <Button size="sm" variant="ghost" onClick={() => copyToClipboard(credentials.host, 'Host')}>
                                                        {copied === 'Host' ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                                    </Button>
                                                </div>
                                            </div>
                                            <div className="space-y-2">
                                                <label className="text-sm font-medium text-muted-foreground">Port</label>
                                                <div className="flex items-center gap-2">
                                                    <code className="flex-1 px-3 py-2 bg-muted rounded text-sm">{credentials.port}</code>
                                                    <Button size="sm" variant="ghost" onClick={() => copyToClipboard(credentials.port.toString(), 'Port')}>
                                                        {copied === 'Port' ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                                    </Button>
                                                </div>
                                            </div>
                                            <div className="space-y-2">
                                                <label className="text-sm font-medium text-muted-foreground">Username</label>
                                                <div className="flex items-center gap-2">
                                                    <code className="flex-1 px-3 py-2 bg-muted rounded text-sm">{credentials.username}</code>
                                                    <Button size="sm" variant="ghost" onClick={() => copyToClipboard(credentials.username, 'Username')}>
                                                        {copied === 'Username' ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                                    </Button>
                                                </div>
                                            </div>
                                            <div className="space-y-2">
                                                <label className="text-sm font-medium text-muted-foreground">Password</label>
                                                <div className="flex items-center gap-2">
                                                    <code className="flex-1 px-3 py-2 bg-muted rounded text-sm font-mono">
                                                        {passwordVisible ? credentials.password : '••••••••••••'}
                                                    </code>
                                                    <Button size="sm" variant="ghost" onClick={() => setPasswordVisible(!passwordVisible)}>
                                                        {passwordVisible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                                    </Button>
                                                    <Button size="sm" variant="ghost" onClick={() => copyToClipboard(credentials.password, 'Password')}>
                                                        {copied === 'Password' ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                                    </Button>
                                                </div>
                                            </div>
                                            <div className="space-y-2 md:col-span-2">
                                                <label className="text-sm font-medium text-muted-foreground">Database</label>
                                                <div className="flex items-center gap-2">
                                                    <code className="flex-1 px-3 py-2 bg-muted rounded text-sm">{credentials.database}</code>
                                                    <Button size="sm" variant="ghost" onClick={() => copyToClipboard(credentials.database, 'Database')}>
                                                        {copied === 'Database' ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                                    </Button>
                                                </div>
                                            </div>
                                        </div>
                                    ) : (
                                        <Button onClick={fetchCredentials} disabled={loadingCredentials}>
                                            {loadingCredentials ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : null}
                                            Load Credentials
                                        </Button>
                                    )}
                                </CardContent>
                            </Card>

                            {connectionExamples.length > 0 && (
                                <Card>
                                    <CardHeader>
                                        <CardTitle className="text-base">Connection Examples</CardTitle>
                                    </CardHeader>
                                    <CardContent>
                                        <Tabs value={connectionExamples[selectedExampleTab]?.title || connectionExamples[0]?.title} onValueChange={(v) => {
                                            const idx = connectionExamples.findIndex(e => e.title === v);
                                            if (idx >= 0) setSelectedExampleTab(idx);
                                        }}>
                                            <TabsList className="flex flex-wrap h-auto gap-1 mb-4 bg-muted/50">
                                                {connectionExamples.map((example) => (
                                                    <TabsTrigger
                                                        key={example.title}
                                                        value={example.title}
                                                        className="text-xs data-[state=active]:bg-white data-[state=active]:text-black"
                                                    >
                                                        {example.title}
                                                    </TabsTrigger>
                                                ))}
                                            </TabsList>
                                            {connectionExamples.map((example) => (
                                                <TabsContent key={example.title} value={example.title} className="mt-0">
                                                    <div className="space-y-2">
                                                        <p className="text-sm text-muted-foreground">{example.description}</p>
                                                        <div className="relative">
                                                            <pre className="px-4 py-3 bg-zinc-900 text-zinc-100 rounded-lg text-sm overflow-x-auto">
                                                                <code>{example.code}</code>
                                                            </pre>
                                                            <Button
                                                                size="sm"
                                                                variant="ghost"
                                                                className="absolute top-2 right-2 h-7 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800"
                                                                onClick={() => copyToClipboard(example.code, example.title)}
                                                            >
                                                                {copied === example.title ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                                            </Button>
                                                        </div>
                                                    </div>
                                                </TabsContent>
                                            ))}
                                        </Tabs>
                                    </CardContent>
                                </Card>
                            )}
                        </TabsContent>

                        {/* Backups Tab */}
                        <TabsContent value="backups" className="space-y-6">
                            <div className="flex items-center justify-between">
                                <div>
                                    <h2 className="text-lg font-semibold">Backup History</h2>
                                    <p className="text-sm text-muted-foreground">Manage your database backups</p>
                                </div>
                                <Button onClick={handleBackup} disabled={actionLoading === 'backup'}>
                                    {actionLoading === 'backup' ? (
                                        <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    ) : (
                                        <Download className="w-4 h-4 mr-2" />
                                    )}
                                    Create Backup
                                </Button>
                            </div>

                            {backups.length > 0 ? (
                                <Card>
                                    <CardContent className="p-0">
                                        <div className="divide-y divide-border">
                                            {backups.map((backup) => (
                                                <div key={backup.id} className="flex items-center justify-between p-4">
                                                    <div>
                                                        <p className="font-medium">{formatDate(backup.createdAt)}</p>
                                                        <p className="text-sm text-muted-foreground">{formatBytes(backup.size)}</p>
                                                    </div>
                                                    <div className="flex items-center gap-2">
                                                        <Badge variant={backup.status === 'completed' ? 'default' : backup.status === 'in-progress' ? 'secondary' : 'destructive'}>
                                                            {backup.status}
                                                        </Badge>
                                                        {backup.status === 'completed' && (
                                                            <>
                                                                <Button
                                                                    size="sm"
                                                                    variant="outline"
                                                                    onClick={() => api.downloadBackup(backup.id)}
                                                                >
                                                                    <Download className="w-4 h-4" />
                                                                </Button>
                                                                <Button
                                                                    size="sm"
                                                                    variant="outline"
                                                                    onClick={() => handlePreviewBackup(backup.id)}
                                                                    disabled={loadingPreview}
                                                                >
                                                                    {loadingPreview ? (
                                                                        <Loader2 className="w-4 h-4 animate-spin" />
                                                                    ) : (
                                                                        <RotateCcw className="w-4 h-4" />
                                                                    )}
                                                                </Button>
                                                                <Button
                                                                    size="sm"
                                                                    variant="outline"
                                                                    className="text-destructive"
                                                                    onClick={() => handleDeleteBackup(backup.id)}
                                                                    disabled={actionLoading === `delete-backup-${backup.id}`}
                                                                >
                                                                    {actionLoading === `delete-backup-${backup.id}` ? (
                                                                        <Loader2 className="w-4 h-4 animate-spin" />
                                                                    ) : (
                                                                        <Trash2 className="w-4 h-4" />
                                                                    )}
                                                                </Button>
                                                            </>
                                                        )}
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    </CardContent>
                                </Card>
                            ) : (
                                <Card>
                                    <CardContent className="flex flex-col items-center justify-center py-12">
                                        <Database className="w-12 h-12 text-muted-foreground mb-4" />
                                        <p className="text-muted-foreground">No backups yet</p>
                                        <p className="text-sm text-muted-foreground">Create a backup to protect your data</p>
                                    </CardContent>
                                </Card>
                            )}
                        </TabsContent>

                        {/* Settings Tab */}
                        <TabsContent value="settings" className="space-y-6">
                            <Card>
                                <CardHeader>
                                    <CardTitle className="text-base">Backup Schedule</CardTitle>
                                </CardHeader>
                                <CardContent className="space-y-4">
                                    <div className="flex items-center justify-between">
                                        <div>
                                            <p className="font-medium">Automatic Backups</p>
                                            <p className="text-sm text-muted-foreground">Enable scheduled backups for this database</p>
                                        </div>
                                        <Switch
                                            checked={backupEnabled}
                                            onCheckedChange={setBackupEnabled}
                                        />
                                    </div>

                                    {backupEnabled && (
                                        <>
                                            <div className="grid gap-4 md:grid-cols-2">
                                                <div>
                                                    <label className="text-sm font-medium">Schedule</label>
                                                    <select
                                                        value={backupSchedule}
                                                        onChange={(e) => setBackupSchedule(e.target.value)}
                                                        className="w-full mt-1 px-3 py-2 border rounded-md bg-background"
                                                    >
                                                        <option value="0 0 * * *">Daily at midnight</option>
                                                        <option value="0 0 * * 0">Weekly (Sunday midnight)</option>
                                                        <option value="0 0 1 * *">Monthly (1st at midnight)</option>
                                                        <option value="0 */6 * * *">Every 6 hours</option>
                                                        <option value="0 */12 * * *">Every 12 hours</option>
                                                    </select>
                                                </div>
                                                <div>
                                                    <label className="text-sm font-medium">Retention (keep last N backups)</label>
                                                    <input
                                                        type="number"
                                                        value={backupRetention}
                                                        onChange={(e) => setBackupRetention(Number(e.target.value))}
                                                        min={1}
                                                        max={100}
                                                        className="w-full mt-1 px-3 py-2 border rounded-md bg-background"
                                                    />
                                                </div>
                                            </div>
                                        </>
                                    )}

                                    <Button onClick={handleSaveBackupSettings} disabled={savingBackupSettings}>
                                        {savingBackupSettings ? (
                                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                        ) : null}
                                        Save Backup Settings
                                    </Button>
                                </CardContent>
                            </Card>

                            {/* Resource Settings */}
                            <Card>
                                <CardHeader>
                                    <CardTitle className="text-base">Resource Limits</CardTitle>
                                </CardHeader>
                                <CardContent className="space-y-4">
                                    <p className="text-sm text-muted-foreground">
                                        Adjust memory and CPU limits for this database. Changes take effect immediately.
                                    </p>
                                    <div className="grid gap-4 md:grid-cols-2">
                                        <div>
                                            <label className="text-sm font-medium">Memory Limit</label>
                                            <select
                                                value={memoryLimit}
                                                onChange={(e) => setMemoryLimit(Number(e.target.value))}
                                                className="w-full mt-1 px-3 py-2 border rounded-md bg-background"
                                            >
                                                <option value={128}>128 MB</option>
                                                <option value={256}>256 MB</option>
                                                <option value={512}>512 MB</option>
                                                <option value={1024}>1 GB</option>
                                                <option value={2048}>2 GB</option>
                                                <option value={4096}>4 GB</option>
                                            </select>
                                        </div>
                                        <div>
                                            <label className="text-sm font-medium">CPU Limit (cores)</label>
                                            <select
                                                value={cpuLimit}
                                                onChange={(e) => setCpuLimit(Number(e.target.value))}
                                                className="w-full mt-1 px-3 py-2 border rounded-md bg-background"
                                            >
                                                <option value={0.25}>0.25 cores</option>
                                                <option value={0.5}>0.5 cores</option>
                                                <option value={1}>1 core</option>
                                                <option value={2}>2 cores</option>
                                                <option value={4}>4 cores</option>
                                            </select>
                                        </div>
                                    </div>

                                    <Button onClick={handleSaveResources} disabled={savingResources}>
                                        {savingResources ? (
                                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                        ) : null}
                                        Update Resources
                                    </Button>
                                </CardContent>
                            </Card>

                            <Card>
                                <CardHeader>
                                    <CardTitle className="text-base">Database Information</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <div className="grid gap-4 md:grid-cols-2">
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Database ID</p>
                                            <code className="text-sm">{database.id}</code>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Container ID</p>
                                            <code className="text-sm">{database.containerId || 'N/A'}</code>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-muted-foreground">Engine</p>
                                            <p className="text-sm">{engineConfig[database.engine]?.name || database.engine} {database.version}</p>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>

                            <Card className="border-destructive">
                                <CardHeader>
                                    <CardTitle className="text-base text-destructive">Danger Zone</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <p className="text-sm text-muted-foreground mb-4">
                                        Once you delete a database, there is no going back. Please be certain.
                                    </p>
                                    <Button
                                        variant="destructive"
                                        onClick={handleDelete}
                                        disabled={actionLoading === 'delete'}
                                    >
                                        {actionLoading === 'delete' ? (
                                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                        ) : (
                                            <Trash2 className="w-4 h-4 mr-2" />
                                        )}
                                        Delete This Database
                                    </Button>
                                </CardContent>
                            </Card>
                        </TabsContent>
                    </Tabs>
                </div>
            </main>
            <CreateDatabaseModal open={createModalOpen} onOpenChange={setCreateModalOpen} onCreate={fetchData} />

            {/* Backup Preview Modal */}
            <Dialog open={previewBackup !== null} onOpenChange={() => setPreviewBackup(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>Restore Backup</DialogTitle>
                        <DialogDescription>
                            Review the backup details before restoring.
                        </DialogDescription>
                    </DialogHeader>
                    {previewBackup && (
                        <div className="space-y-4">
                            <div className="grid grid-cols-2 gap-4 text-sm">
                                <div>
                                    <p className="text-muted-foreground">Database</p>
                                    <p className="font-medium">{previewBackup.databaseName}</p>
                                </div>
                                <div>
                                    <p className="text-muted-foreground">Engine</p>
                                    <p className="font-medium">{previewBackup.engine || 'Unknown'}</p>
                                </div>
                                <div>
                                    <p className="text-muted-foreground">Size</p>
                                    <p className="font-medium">{formatBytes(previewBackup.size)}</p>
                                </div>
                                <div>
                                    <p className="text-muted-foreground">Created</p>
                                    <p className="font-medium">{formatDate(previewBackup.createdAt)}</p>
                                </div>
                            </div>
                            <Alert>
                                <AlertCircle className="h-4 w-4" />
                                <AlertDescription>
                                    Restoring this backup will replace all current data in the database.
                                    This action cannot be undone.
                                </AlertDescription>
                            </Alert>
                        </div>
                    )}
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setPreviewBackup(null)}>
                            Cancel
                        </Button>
                        <Button onClick={handleConfirmRestore} disabled={actionLoading === 'restore'}>
                            {actionLoading === 'restore' ? (
                                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                            ) : null}
                            Restore Backup
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    );
}
