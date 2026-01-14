"use client";

import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useAuth } from "@/components/AuthProvider";
import { cn } from "@/lib/utils";
import {
    Database,
    LayoutDashboard,
    Archive,
    ChevronLeft,
    ChevronRight,
    Plus,
    Boxes,
    Network,
    LogOut,
    User,
} from "lucide-react";

interface SidebarProps {
    onCreateDatabase?: () => void;
}

const navigation = [
    { name: "Dashboard", href: "/", icon: LayoutDashboard },
    { name: "Databases", href: "/databases", icon: Database },
    { name: "Topology", href: "/topology", icon: Network },
    { name: "Backups", href: "/backups", icon: Archive },
];

export function Sidebar({ onCreateDatabase }: SidebarProps) {
    const location = useLocation();
    const navigate = useNavigate();
    const pathname = location.pathname;
    const [collapsed, setCollapsed] = useState(false);
    const { user, isAuthenticated, authEnabled, logout } = useAuth();

    const handleLogout = async () => {
        await logout();
        navigate("/login");
    };

    return (
        <aside
            className={cn(
                "flex flex-col h-screen bg-sidebar border-r border-sidebar-border transition-all duration-300 shrink-0",
                collapsed ? "w-16" : "w-64"
            )}
        >
            {/* Header */}
            <div className="flex items-center h-16 px-4 border-b border-sidebar-border">
                <div className="flex items-center gap-3 flex-1 min-w-0">
                    <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-foreground/10">
                        <Boxes className="w-5 h-5" />
                    </div>
                    {!collapsed && (
                        <span className="font-bold text-lg">
                            DBnest
                        </span>
                    )}
                </div>
                <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 shrink-0"
                    onClick={() => setCollapsed(!collapsed)}
                >
                    {collapsed ? (
                        <ChevronRight className="w-4 h-4" />
                    ) : (
                        <ChevronLeft className="w-4 h-4" />
                    )}
                </Button>
            </div>

            {/* Create Database */}
            <div className="p-3">
                <Button
                    onClick={onCreateDatabase}
                    className={cn(
                        "w-full",
                        collapsed && "px-0"
                    )}
                >
                    <Plus className="w-4 h-4" />
                    {!collapsed && <span className="ml-2">Create Database</span>}
                </Button>
            </div>

            {/* Navigation */}
            <nav className="flex-1 p-3 space-y-1">
                {navigation.map((item) => {
                    const isActive =
                        pathname === item.href ||
                        (item.href !== "/" && pathname.startsWith(item.href));

                    return (
                        <Link
                            key={item.name}
                            to={item.href}
                            className={cn(
                                "flex items-center gap-3 px-3 py-2.5 rounded-lg transition-all duration-200",
                                isActive
                                    ? "bg-foreground/10 text-foreground font-medium"
                                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                                collapsed && "justify-center px-0"
                            )}
                        >
                            <item.icon className={cn("w-5 h-5 shrink-0")} />
                            {!collapsed && <span>{item.name}</span>}
                        </Link>
                    );
                })}
            </nav>

            {/* Footer */}
            <div className="p-3 border-t border-sidebar-border space-y-2">
                {/* User info and logout (when auth is enabled) */}
                {authEnabled && isAuthenticated && user && (
                    <div className={cn(
                        "flex items-center gap-2",
                        collapsed ? "flex-col" : "justify-between"
                    )}>
                        <div className={cn(
                            "flex items-center gap-2 min-w-0",
                            collapsed && "justify-center"
                        )}>
                            <div className="flex items-center justify-center w-7 h-7 rounded-full bg-primary/10 shrink-0">
                                <User className="w-4 h-4 text-primary" />
                            </div>
                            {!collapsed && (
                                <span className="text-sm font-medium truncate">
                                    {user.username}
                                </span>
                            )}
                        </div>
                        <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7 shrink-0"
                            onClick={handleLogout}
                            title="Logout"
                        >
                            <LogOut className="w-4 h-4 text-muted-foreground" />
                        </Button>
                    </div>
                )}

                {/* Version and theme toggle */}
                <div className={cn("flex items-center", collapsed ? "justify-center" : "justify-between")}>
                    {!collapsed && (
                        <span className="text-xs text-muted-foreground">v1.0.0</span>
                    )}
                    <ThemeToggle />
                </div>
            </div>
        </aside>
    );
}
