"use client";

import { useState, useEffect } from "react";
import { api, CreateDatabaseRequest, DockerNetwork } from "@/lib/api";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { Database, ArrowRight, ArrowLeft, Loader2, Box } from "lucide-react";

interface CreateDatabaseModalProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCreate?: () => void;
}

type DatabaseEngine = "postgresql" | "mysql" | "mariadb" | "redis";

const engineConfig: Record<DatabaseEngine, {
    name: string;
    versions: string[];
}> = {
    postgresql: {
        name: "PostgreSQL",
        versions: ["16", "15", "14", "13"],
    },
    mysql: {
        name: "MySQL",
        versions: ["8.0", "8.4", "5.7"],
    },
    mariadb: {
        name: "MariaDB",
        versions: ["11", "10.11", "10.6"],
    },
    redis: {
        name: "Redis",
        versions: ["7", "7.2", "6"],
    },
};

interface FormData {
    name: string;
    engine: DatabaseEngine;
    version: string;
    username: string;
    password: string;
    database: string;
    storageLimit: number;
    memoryLimit: number;
    network: string;
    exposePort: boolean;
    backupEnabled: boolean;
    backupSchedule: string;
    backupRetentionCount: number;
    seedSource: 'none' | 'text' | 'file';
    seedContent: string;
}

export function CreateDatabaseModal({
    open,
    onOpenChange,
    onCreate,
}: CreateDatabaseModalProps) {
    const [step, setStep] = useState<1 | 2 | 3>(1);
    const [isCreating, setIsCreating] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [networks, setNetworks] = useState<DockerNetwork[]>([]);
    const [isCreatingNetwork, setIsCreatingNetwork] = useState(false);
    const [newNetworkName, setNewNetworkName] = useState("");
    const [formData, setFormData] = useState<FormData>({
        name: "",
        engine: "postgresql",
        version: "16",
        username: "admin",
        password: "",
        database: "",
        storageLimit: 1024,
        memoryLimit: 256,
        network: "",
        exposePort: true,
        backupEnabled: false,
        backupSchedule: "0 2 * * *",
        backupRetentionCount: 5,
        seedSource: 'none',
        seedContent: '',
    });

    // Fetch networks when modal opens
    useEffect(() => {
        if (open) {
            api.listNetworks().then(setNetworks).catch(console.error);
        }
    }, [open]);

    const handleEngineSelect = (engine: DatabaseEngine) => {
        setFormData({
            ...formData,
            engine,
            version: engineConfig[engine].versions[0],
        });
        setStep(2);
    };

    const handleCreate = async () => {
        setIsCreating(true);
        setError(null);

        try {
            const request: CreateDatabaseRequest = {
                name: formData.name,
                engine: formData.engine,
                version: formData.version,
                username: formData.username,
                password: formData.password || undefined,
                database: formData.database,
                storageLimit: formData.storageLimit,
                memoryLimit: formData.memoryLimit,
                network: formData.network || undefined,
                exposePort: formData.exposePort,
                backupEnabled: formData.backupEnabled,
                backupSchedule: formData.backupSchedule,
                backupRetentionCount: formData.backupRetentionCount,
                seedSource: formData.seedSource,
                seedContent: formData.seedContent,
            };

            await api.createDatabase(request);
            handleClose();
            onCreate?.();
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to create database");
        } finally {
            setIsCreating(false);
        }
    };

    const handleClose = () => {
        onOpenChange(false);
        setStep(1);
        setError(null);
        setFormData({
            name: "",
            engine: "postgresql",
            version: "16",
            username: "admin",
            password: "",
            database: "",
            storageLimit: 1024,
            memoryLimit: 256,
            network: "",
            exposePort: true,
            backupEnabled: false,
            backupSchedule: "0 2 * * *",
            backupRetentionCount: 5,
            seedSource: 'none',
            seedContent: '',
        });
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-2xl">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <Database className="w-5 h-5" />
                        Create New Database
                    </DialogTitle>
                    <DialogDescription>
                        {step === 1 && "Choose your database engine"}
                        {step === 2 && "Configure your database settings"}
                        {step === 3 && "Review and confirm"}
                    </DialogDescription>
                </DialogHeader>

                {/* Step indicator */}
                <div className="flex items-center justify-center gap-2 py-4">
                    {[1, 2, 3].map((s) => (
                        <div
                            key={s}
                            className={cn(
                                "w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-colors",
                                step >= s
                                    ? "bg-foreground text-background"
                                    : "bg-muted text-muted-foreground"
                            )}
                        >
                            {s}
                        </div>
                    ))}
                </div>

                {error && (
                    <div className="bg-destructive/10 text-destructive p-3 rounded-md text-sm">
                        {error}
                    </div>
                )}

                {/* Step 1: Engine Selection */}
                {step === 1 && (
                    <div className="grid grid-cols-2 gap-4">
                        {(Object.keys(engineConfig) as DatabaseEngine[]).map((engine) => (
                            <button
                                key={engine}
                                onClick={() => handleEngineSelect(engine)}
                                className={cn(
                                    "p-4 rounded-lg border-2 text-left transition-colors hover:border-foreground",
                                    formData.engine === engine
                                        ? "border-foreground bg-muted"
                                        : "border-border"
                                )}
                            >
                                <div className="flex items-center gap-3">
                                    <Box className="w-8 h-8" />
                                    <div>
                                        <p className="font-medium">{engineConfig[engine].name}</p>
                                        <p className="text-sm text-muted-foreground">
                                            v{engineConfig[engine].versions[0]}
                                        </p>
                                    </div>
                                </div>
                            </button>
                        ))}
                    </div>
                )}

                {/* Step 2: Database Configuration */}
                {step === 2 && (
                    <div className="space-y-4">
                        <div className="grid grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="name">Container Name</Label>
                                <Input
                                    id="name"
                                    value={formData.name}
                                    onChange={(e) =>
                                        setFormData({ ...formData, name: e.target.value })
                                    }
                                    placeholder="my-production-db"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="version">Version</Label>
                                <Select
                                    value={formData.version}
                                    onValueChange={(v) =>
                                        setFormData({ ...formData, version: v })
                                    }
                                >
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {engineConfig[formData.engine].versions.map((v) => (
                                            <SelectItem key={v} value={v}>
                                                {v}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="username">Username</Label>
                                <Input
                                    id="username"
                                    value={formData.username}
                                    onChange={(e) =>
                                        setFormData({ ...formData, username: e.target.value })
                                    }
                                    placeholder="admin"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="password">Password (optional)</Label>
                                <Input
                                    id="password"
                                    type="password"
                                    value={formData.password}
                                    onChange={(e) =>
                                        setFormData({ ...formData, password: e.target.value })
                                    }
                                    placeholder="Auto-generated if empty"
                                />
                            </div>
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="database">Database Name</Label>
                            <Input
                                id="database"
                                value={formData.database}
                                onChange={(e) =>
                                    setFormData({ ...formData, database: e.target.value })
                                }
                                placeholder="app_production_db"
                            />
                            <p className="text-xs text-muted-foreground">
                                The initial SQL database to create inside the container
                            </p>
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="storage">Storage Limit (MB)</Label>
                                <Input
                                    id="storage"
                                    type="number"
                                    value={formData.storageLimit}
                                    onChange={(e) =>
                                        setFormData({
                                            ...formData,
                                            storageLimit: parseInt(e.target.value) || 1024,
                                        })
                                    }
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="memory">Memory Limit (MB)</Label>
                                <Input
                                    id="memory"
                                    type="number"
                                    value={formData.memoryLimit}
                                    onChange={(e) =>
                                        setFormData({
                                            ...formData,
                                            memoryLimit: parseInt(e.target.value) || 256,
                                        })
                                    }
                                />
                            </div>
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="network">Docker Network (optional)</Label>
                            {isCreatingNetwork ? (
                                <div className="flex gap-2">
                                    <Input
                                        placeholder="Network name (without dbnest- prefix)"
                                        value={newNetworkName}
                                        onChange={(e) => setNewNetworkName(e.target.value)}
                                    />
                                    <Button
                                        type="button"
                                        size="sm"
                                        onClick={async () => {
                                            if (!newNetworkName) return;
                                            try {
                                                const network = await api.createNetwork(newNetworkName);
                                                setNetworks([...networks, network]);
                                                setFormData({ ...formData, network: network.name });
                                                setNewNetworkName("");
                                                setIsCreatingNetwork(false);
                                            } catch (err) {
                                                setError(err instanceof Error ? err.message : "Failed to create network");
                                            }
                                        }}
                                        disabled={!newNetworkName}
                                    >
                                        Create
                                    </Button>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        onClick={() => {
                                            setIsCreatingNetwork(false);
                                            setNewNetworkName("");
                                        }}
                                    >
                                        Cancel
                                    </Button>
                                </div>
                            ) : (
                                <Select
                                    value={formData.network}
                                    onValueChange={(v) => {
                                        if (v === "__create__") {
                                            setIsCreatingNetwork(true);
                                        } else {
                                            setFormData({ ...formData, network: v === "none" ? "" : v });
                                        }
                                    }}
                                >
                                    <SelectTrigger>
                                        <SelectValue placeholder="Default network" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="none">Default network</SelectItem>
                                        <SelectItem value="__create__" className="text-primary font-medium">
                                            + Create New Network...
                                        </SelectItem>
                                        {networks.map((n) => (
                                            <SelectItem key={n.id} value={n.name}>
                                                {n.name} ({n.driver})
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            )}
                            <p className="text-xs text-muted-foreground">
                                Connect to a network to enable communication between databases
                            </p>
                        </div>

                        {/* Expose Port Toggle */}
                        <div className="flex items-center justify-between py-2">
                            <div>
                                <Label htmlFor="exposePort">Expose Port to Host</Label>
                                <p className="text-sm text-muted-foreground">
                                    Allow connections from outside Docker network
                                </p>
                            </div>
                            <Switch
                                id="exposePort"
                                checked={formData.exposePort}
                                onCheckedChange={(checked) =>
                                    setFormData({ ...formData, exposePort: checked })
                                }
                            />
                        </div>

                        <div className="border-t pt-4 space-y-4">
                            <div className="flex items-center justify-between">
                                <div>
                                    <Label htmlFor="backup">Enable Backups</Label>
                                    <p className="text-sm text-muted-foreground">
                                        Automatically backup this database
                                    </p>
                                </div>
                                <Switch
                                    id="backup"
                                    checked={formData.backupEnabled}
                                    onCheckedChange={(checked) =>
                                        setFormData({ ...formData, backupEnabled: checked })
                                    }
                                />
                            </div>
                            {formData.backupEnabled && (
                                <div className="grid grid-cols-2 gap-4">
                                    <div className="space-y-2">
                                        <Label htmlFor="schedule">Schedule (cron)</Label>
                                        <Input
                                            id="schedule"
                                            value={formData.backupSchedule}
                                            onChange={(e) =>
                                                setFormData({ ...formData, backupSchedule: e.target.value })
                                            }
                                            placeholder="0 2 * * *"
                                        />
                                    </div>
                                    <div className="space-y-2">
                                        <Label htmlFor="retention">Retention Count</Label>
                                        <Input
                                            id="retention"
                                            type="number"
                                            value={formData.backupRetentionCount}
                                            onChange={(e) =>
                                                setFormData({
                                                    ...formData,
                                                    backupRetentionCount: parseInt(e.target.value) || 5,
                                                })
                                            }
                                        />
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                )}

                {/* Step 3: Review */}
                {step === 3 && (
                    <div className="space-y-4">
                        <div className="bg-muted p-4 rounded-lg space-y-2">
                            <dl className="grid grid-cols-2 gap-2 text-sm">
                                <dt className="text-muted-foreground">Engine</dt>
                                <dd className="font-medium">
                                    {engineConfig[formData.engine].name} {formData.version}
                                </dd>
                                <dt className="text-muted-foreground">Name</dt>
                                <dd className="font-medium">{formData.name}</dd>
                                <dt className="text-muted-foreground">Username</dt>
                                <dd className="font-medium">{formData.username}</dd>
                                <dt className="text-muted-foreground">Database</dt>
                                <dd className="font-medium">{formData.database}</dd>
                                <dt className="text-muted-foreground">Storage</dt>
                                <dd className="font-medium">{formData.storageLimit} MB</dd>
                                <dt className="text-muted-foreground">Memory</dt>
                                <dd className="font-medium">{formData.memoryLimit} MB</dd>
                                {formData.network && (
                                    <>
                                        <dt className="text-muted-foreground">Network</dt>
                                        <dd className="font-medium">{formData.network}</dd>
                                    </>
                                )}
                                <dt className="text-muted-foreground">Backups</dt>
                                <dd className="font-medium">
                                    {formData.backupEnabled ? "Enabled" : "Disabled"}
                                </dd>
                            </dl>
                        </div>
                    </div>
                )}

                <DialogFooter className="flex justify-between">
                    <div>
                        {step > 1 && (
                            <Button
                                variant="outline"
                                onClick={() => setStep((s) => (s > 1 ? (s - 1) as 1 | 2 | 3 : s))}
                            >
                                <ArrowLeft className="w-4 h-4 mr-2" />
                                Back
                            </Button>
                        )}
                    </div>
                    <div className="flex gap-2">
                        <Button variant="outline" onClick={handleClose}>
                            Cancel
                        </Button>
                        {step === 2 && (
                            <Button
                                onClick={() => setStep(3)}
                                disabled={!formData.name || !formData.username || !formData.database}
                            >
                                Next
                                <ArrowRight className="w-4 h-4 ml-2" />
                            </Button>
                        )}
                        {step === 3 && (
                            <Button onClick={handleCreate} disabled={isCreating}>
                                {isCreating && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
                                Create Database
                            </Button>
                        )}
                    </div>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
