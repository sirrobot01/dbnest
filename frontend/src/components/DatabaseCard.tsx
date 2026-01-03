import { useState } from "react";
import { Link } from "react-router-dom";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { StatusBadge } from "@/components/StatusBadge";
import {
    DatabaseInstance,
    engineConfig,
    DatabaseEngine,
} from "@/lib/mockData";

import {
    MoreVertical,
    Play,
    Square,
    Trash2,
    Copy,
    ExternalLink,
    HardDrive,
    Users,
    Database,
    Check,
    Loader2
} from "lucide-react";
import { formatBytes } from "@/lib/utils";

// Engine icons as simple representations
function EngineIcon({ engine: _engine }: { engine: DatabaseEngine }) {
    return (
        <div className="flex items-center justify-center w-10 h-10 rounded-lg bg-muted border border-border">
            <Database className="w-5 h-5" />
        </div>
    );
}

interface DatabaseCardProps {
    database: DatabaseInstance;
    onStart?: (id: string) => Promise<void> | void;
    onStop?: (id: string) => Promise<void> | void;
    onDelete?: (id: string) => Promise<void> | void;
    selected?: boolean;
    onToggleSelect?: (id: string) => void;
}

export function DatabaseCard({
    database,
    onStart,
    onStop,
    onDelete,
    selected = false,
    onToggleSelect,
}: DatabaseCardProps) {
    const [actionLoading, setActionLoading] = useState<string | null>(null);

    const config = engineConfig[database.engine] || {
        name: database.engine,
        color: "text-foreground",
        bgColor: "bg-muted",
        borderColor: "border-border",
        versions: [],
        defaultPort: 0,
    };
    const storagePercent = (database.storageUsed / database.storageLimit) * 100;
    const connectionPercent =
        (database.connections / database.maxConnections) * 100;


    const copyConnectionString = () => {
        let connString = "";
        if (database.engine === "postgresql") {
            connString = `postgresql://${database.username}@${database.host}:${database.port}/${database.database}`;
        } else if (database.engine === "mysql") {
            connString = `mysql://${database.username}@${database.host}:${database.port}/${database.database}`;
        } else {
            connString = database.database;
        }
        navigator.clipboard.writeText(connString);
    };

    const handleStart = async (e: React.MouseEvent) => {
        e.stopPropagation();
        if (onStart) {
            setActionLoading('start');
            try {
                await onStart(database.id);
            } finally {
                setActionLoading(null);
            }
        }
    };

    const handleStop = async (e: React.MouseEvent) => {
        e.stopPropagation();
        if (onStop) {
            setActionLoading('stop');
            try {
                await onStop(database.id);
            } finally {
                setActionLoading(null);
            }
        }
    };

    return (
        <Card className={`group hover:border-foreground/50 transition-all duration-200 hover:shadow-lg ${selected ? 'ring-2 ring-primary border-primary' : ''}`}>
            <CardHeader className="pb-3">
                <div className="flex items-start justify-between">
                    <div className="flex items-center gap-3">
                        {/* Selection checkbox */}
                        {onToggleSelect && (
                            <button
                                onClick={(e) => {
                                    e.stopPropagation();
                                    onToggleSelect(database.id);
                                }}
                                className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-colors ${selected
                                    ? 'bg-primary border-primary text-primary-foreground'
                                    : 'border-muted-foreground/50 hover:border-primary'
                                    }`}
                            >
                                {selected && <Check className="w-3 h-3" />}
                            </button>
                        )}
                        <EngineIcon engine={database.engine} />
                        <div>
                            <CardTitle className="text-base font-semibold">
                                {database.name}
                            </CardTitle>
                            <CardDescription className="text-xs">
                                {config.name} {database.version}
                            </CardDescription>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <StatusBadge status={database.status} />
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-8 w-8"
                                >
                                    <MoreVertical className="w-4 h-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={copyConnectionString}>
                                    <Copy className="w-4 h-4 mr-2" />
                                    Copy Connection String
                                </DropdownMenuItem>
                                <DropdownMenuItem asChild>
                                    <Link to={`/databases/${database.id}`}>
                                        <ExternalLink className="w-4 h-4 mr-2" />
                                        View Details
                                    </Link>
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                {database.status === "stopped" ? (
                                    <DropdownMenuItem onClick={() => onStart?.(database.id)}>
                                        <Play className="w-4 h-4 mr-2" />
                                        Start Database
                                    </DropdownMenuItem>
                                ) : (
                                    <DropdownMenuItem onClick={() => onStop?.(database.id)}>
                                        <Square className="w-4 h-4 mr-2" />
                                        Stop Database
                                    </DropdownMenuItem>
                                )}
                                <DropdownMenuSeparator />
                                <DropdownMenuItem
                                    onClick={() => onDelete?.(database.id)}
                                    className="text-destructive focus:text-destructive"
                                >
                                    <Trash2 className="w-4 h-4 mr-2" />
                                    Delete Database
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </div>
                </div>
            </CardHeader>
            <CardContent className="space-y-4">
                {/* Metrics */}
                <div className="grid grid-cols-2 gap-4">
                    {/* Storage */}
                    <div className="space-y-1.5">
                        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                            <HardDrive className="w-3.5 h-3.5" />
                            <span>Storage</span>
                        </div>
                        <div className="space-y-1">
                            <div className="flex justify-between text-xs">
                                <span className="font-medium">
                                    {formatBytes(database.storageUsed)}
                                </span>
                                <span className="text-muted-foreground">
                                    / {formatBytes(database.storageLimit)}
                                </span>
                            </div>
                            <div className="h-1.5 bg-muted rounded-full overflow-hidden">
                                <div
                                    className="h-full rounded-full transition-all duration-500 bg-foreground"
                                    style={{ width: `${storagePercent}%` }}
                                />
                            </div>
                        </div>
                    </div>

                    {/* Connections */}
                    <div className="space-y-1.5">
                        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                            <Users className="w-3.5 h-3.5" />
                            <span>Connections</span>
                        </div>
                        <div className="space-y-1">
                            <div className="flex justify-between text-xs">
                                <span className="font-medium">{database.connections}</span>
                                <span className="text-muted-foreground">
                                    / {database.maxConnections}
                                </span>
                            </div>
                            <div className="h-1.5 bg-muted rounded-full overflow-hidden">
                                <div
                                    className="h-full rounded-full transition-all duration-500 bg-foreground"
                                    style={{ width: `${connectionPercent}%` }}
                                />
                            </div>
                        </div>
                    </div>
                </div>

                {/* Quick Actions */}
                <div className="flex gap-2 pt-2 border-t border-border">
                    <Button variant="outline" size="sm" className="flex-1" asChild>
                        <Link to={`/databases/${database.id}`}>View Details</Link>
                    </Button>
                    {database.status === "stopped" ? (
                        <Button
                            size="sm"
                            className="flex-1 bg-emerald-600 hover:bg-emerald-700 text-white border-emerald-600"
                            onClick={handleStart}
                            disabled={actionLoading === 'start'}
                        >
                            {actionLoading === 'start' ? (
                                <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" />
                            ) : (
                                <Play className="w-3.5 h-3.5 mr-1.5" />
                            )}
                            Start
                        </Button>
                    ) : (
                        <Button
                            variant="outline"
                            size="sm"
                            className="flex-1 border-amber-500/50 text-amber-600 hover:bg-amber-500/10 hover:text-amber-700 hover:border-amber-500"
                            onClick={handleStop}
                            disabled={database.status === "creating" || actionLoading === 'stop'}
                        >
                            {actionLoading === 'stop' ? (
                                <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" />
                            ) : (
                                <Square className="w-3.5 h-3.5 mr-1.5 fill-current" />
                            )}
                            Stop
                        </Button>
                    )}
                </div>
            </CardContent>
        </Card>
    );
}
