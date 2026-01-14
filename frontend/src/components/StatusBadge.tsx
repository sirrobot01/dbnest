"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { DatabaseStatus, statusConfig } from "@/lib/mockData";

interface StatusBadgeProps {
    status: DatabaseStatus;
    showDot?: boolean;
    className?: string;
}

export function StatusBadge({
    status,
    showDot = true,
    className,
}: StatusBadgeProps) {
    const config = statusConfig[status];

    return (
        <Badge
            variant="outline"
            className={cn(
                "font-medium border-0",
                config.bgColor,
                config.color,
                className
            )}
        >
            {showDot && (
                <span
                    className={cn(
                        "w-1.5 h-1.5 rounded-full mr-1.5",
                        config.dotColor,
                        status === "running" && "animate-pulse"
                    )}
                />
            )}
            {config.label}
        </Badge>
    );
}
