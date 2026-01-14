"use client";

import { useState, useMemo } from "react";
import { Link } from "react-router-dom";
import { TopologyNetwork } from "@/lib/api";
import { Card } from "@/components/ui/card";
import {
    Database,
    Server,
    Loader2,
    AlertCircle,
    Globe
} from "lucide-react";
import { cn } from "@/lib/utils";

interface NetworkTopologyProps {
    className?: string;
    topology: TopologyNetwork[];
    loading: boolean;
    error: string | null;
}

interface Point {
    x: number;
    y: number;
}

interface Node {
    id: string;
    type: 'host' | 'network' | 'database';
    x: number;
    y: number;
    data: any;
    label: string;
    status?: string;
}

const statusColors: Record<string, string> = {
    running: "bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]",
    stopped: "bg-amber-500",
    creating: "bg-yellow-500 animate-pulse",
    error: "bg-red-500 animate-pulse",
};

export function NetworkTopology({ className, topology, loading, error }: NetworkTopologyProps) {
    const [hoveredNode, setHoveredNode] = useState<string | null>(null);

    // Canvas dimensions
    const width = 800;
    const height = 600;
    const centerX = width / 2;


    // Layout calculation (Tree/Hierarchy)
    const graph = useMemo(() => {
        const nodes: Node[] = [];
        const links: { source: Point; target: Point; sourceId: string; targetId: string }[] = [];

        // 1. Host Node (Top Center)
        const hostNode: Node = {
            id: 'host',
            type: 'host',
            x: centerX,
            y: 80, // Fixed top position
            label: 'DBnest Host',
            data: {}
        };
        nodes.push(hostNode);

        if (topology.length === 0) return { nodes, links };

        // 2. Calculate columns for Networks
        // Divide the width into equal columns for each network
        const columnCount = topology.length;
        const columnWidth = width / columnCount;

        topology.forEach((net, i) => {
            // Network is centered in its column
            const netX = (i * columnWidth) + (columnWidth / 2);
            const netY = 250; // Fixed middle tier

            const netNode: Node = {
                id: `net-${net.name}`,
                type: 'network',
                x: netX,
                y: netY,
                label: net.name === 'default' ? 'Default Network' : net.name,
                data: net
            };
            nodes.push(netNode);
            links.push({
                source: { x: hostNode.x, y: hostNode.y },
                target: { x: netX, y: netY },
                sourceId: hostNode.id,
                targetId: netNode.id
            });

            // 3. Databases (Bottom tier, distributed within network column)
            const dbs = net.databases || [];
            if (dbs.length === 0) return;

            // Distribute DBs evenly within the network's column width
            // We use 80% of the column width to leave some padding between networks
            const availableWidth = columnWidth * 0.8;
            const dbSpacing = availableWidth / (dbs.length + 1);
            const startX = netX - (availableWidth / 2);

            dbs.forEach((db, j) => {
                const dbX = startX + (dbSpacing * (j + 1));
                const dbY = 450; // Fixed bottom tier

                const dbNode: Node = {
                    id: db.id,
                    type: 'database',
                    x: dbX,
                    y: dbY,
                    label: db.name,
                    status: db.status,
                    data: db
                };
                nodes.push(dbNode);
                links.push({
                    source: { x: netX, y: netY },
                    target: { x: dbX, y: dbY },
                    sourceId: netNode.id,
                    targetId: dbNode.id
                });
            });
        });

        return { nodes, links };
    }, [topology, width, centerX]);

    if (loading) {
        return (
            <div className={cn("flex items-center justify-center p-12 h-[600px]", className)}>
                <Loader2 className="w-8 h-8 animate-spin text-primary" />
            </div>
        );
    }

    if (error) {
        return (
            <div className={cn("flex items-center justify-center p-12 h-[600px]", className)}>
                <div className="text-center text-muted-foreground">
                    <AlertCircle className="w-8 h-8 mx-auto mb-2 opacity-50" />
                    <p>{error}</p>
                </div>
            </div>
        );
    }

    return (
        <Card className={cn("overflow-hidden bg-zinc-950/50 backdrop-blur-sm border-zinc-800", className)}>
            <div className="relative mx-auto" style={{ width, height }}>
                {/* SVG Layer for Connections */}
                <svg
                    className="absolute inset-0 w-full h-full pointer-events-none"
                    viewBox={`0 0 ${width} ${height}`}
                >
                    <defs>
                        <linearGradient id="link-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
                            <stop offset="0%" stopColor="rgba(255,255,255,0.05)" />
                            <stop offset="100%" stopColor="rgba(255,255,255,0.2)" />
                        </linearGradient>
                    </defs>
                    {graph.links.map((link, i) => {
                        const isHovered = hoveredNode === link.sourceId || hoveredNode === link.targetId;
                        return (
                            <path
                                key={i}
                                d={`M ${link.source.x} ${link.source.y} C ${link.source.x} ${link.source.y + 60}, ${link.target.x} ${link.target.y - 60}, ${link.target.x} ${link.target.y}`}
                                stroke={isHovered ? "rgba(255,255,255,0.4)" : "rgba(255,255,255,0.1)"}
                                strokeWidth={isHovered ? 2 : 1}
                                fill="none"
                                className="transition-all duration-300"
                            />
                        );
                    })}
                </svg>

                {/* HTML Layer for Nodes */}
                <div className="absolute inset-0 w-full h-full">
                    {graph.nodes.map((node) => {
                        // Node Positioning
                        const style = {
                            left: node.x,
                            top: node.y,
                            transform: 'translate(-50%, -50%)'
                        };

                        if (node.type === 'host') {
                            return (
                                <div
                                    key={node.id}
                                    className="absolute flex flex-col items-center justify-center z-20 group cursor-default"
                                    style={style}
                                    onMouseEnter={() => setHoveredNode(node.id)}
                                    onMouseLeave={() => setHoveredNode(null)}
                                >
                                    <div className="w-20 h-20 rounded-full bg-zinc-900 border-4 border-zinc-800 flex items-center justify-center shadow-xl group-hover:scale-105 group-hover:border-primary/50 transition-all duration-300">
                                        <Server className="w-8 h-8 text-zinc-400 group-hover:text-primary transition-colors" />
                                    </div>
                                    <div className="mt-3 px-3 py-1 rounded-full bg-zinc-900/80 border border-zinc-800 text-xs font-medium backdrop-blur-md">
                                        DBnest Host
                                    </div>
                                </div>
                            );
                        }

                        if (node.type === 'network') {
                            return (
                                <div
                                    key={node.id}
                                    className="absolute flex flex-col items-center justify-center z-10 group cursor-default"
                                    style={style}
                                    onMouseEnter={() => setHoveredNode(node.id)}
                                    onMouseLeave={() => setHoveredNode(null)}
                                >
                                    <div className="relative">
                                        <div className="w-14 h-14 rounded-full bg-indigo-950/30 border-2 border-indigo-500/20 flex items-center justify-center backdrop-blur-sm group-hover:border-indigo-500/60 group-hover:bg-indigo-900/40 transition-all duration-300 shadow-lg">
                                            <Globe className="w-6 h-6 text-indigo-400 group-hover:text-indigo-300" />
                                        </div>
                                        {/* Pulse effect */}
                                        <div className="absolute inset-0 rounded-full bg-indigo-500/10 animate-ping opacity-0 group-hover:opacity-100 duration-1000" />
                                    </div>
                                    <div className="mt-2 px-3 py-1 rounded-full bg-zinc-900/80 border border-zinc-800 text-xs text-indigo-200 font-medium backdrop-blur-md whitespace-nowrap">
                                        {node.label}
                                    </div>
                                </div>
                            );
                        }

                        if (node.type === 'database') {
                            return (
                                <Link
                                    key={node.id}
                                    to={`/databases/${node.id}`}
                                    className="absolute flex flex-col items-center justify-center z-30 group"
                                    style={style}
                                    onMouseEnter={() => setHoveredNode(node.id)}
                                    onMouseLeave={() => setHoveredNode(null)}
                                >
                                    <div className="relative">
                                        {/* Status Ring */}
                                        <div className={cn(
                                            "absolute -inset-1 rounded-full opacity-0 group-hover:opacity-100 transition-opacity duration-300 blur-sm",
                                            statusColors[node.status || 'stopped']
                                        )} />

                                        <div className={cn(
                                            "relative w-12 h-12 rounded-xl flex items-center justify-center shadow-lg transition-all duration-300 transform group-hover:-translate-y-1",
                                            "bg-zinc-900 border border-zinc-700 group-hover:border-zinc-500"
                                        )}>
                                            <Database className={cn(
                                                "w-5 h-5 transition-colors",
                                                node.status === 'running' ? "text-green-400" : "text-zinc-500"
                                            )} />

                                            {/* Status Dot */}
                                            <div className={cn(
                                                "absolute -top-1 -right-1 w-3 h-3 rounded-full border-2 border-zinc-950",
                                                statusColors[node.status || 'stopped']
                                            )} />
                                        </div>
                                    </div>

                                    {/* Label */}
                                    <div className="mt-2 px-2 py-0.5 rounded bg-zinc-950/50 backdrop-blur-sm border border-transparent group-hover:border-zinc-800 transition-colors">
                                        <p className="text-xs font-medium text-zinc-400 group-hover:text-zinc-100 max-w-[120px] truncate text-center">
                                            {node.label}
                                        </p>
                                    </div>
                                </Link>
                            );
                        }
                    })}
                </div>

                {/* Overlay Controls / Zoom (Optional Future Add) */}
                <div className="absolute bottom-4 right-4 text-xs text-zinc-600">
                    Live Topology
                </div>
            </div>
        </Card>
    );
}
