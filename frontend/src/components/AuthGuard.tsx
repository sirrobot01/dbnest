"use client";

import { useAuth } from "@/components/AuthProvider";
import { useLocation, useNavigate } from "react-router-dom";
import { useEffect } from "react";

interface AuthGuardProps {
    children: React.ReactNode;
}

export function AuthGuard({ children }: AuthGuardProps) {
    const { isLoading, isAuthenticated, authEnabled, authConfigured } = useAuth();
    const navigate = useNavigate();
    const location = useLocation();
    const pathname = location.pathname;

    useEffect(() => {
        if (isLoading) return;

        // If auth is not enabled, allow all access
        if (!authEnabled) return;

        // If on login page, don't redirect
        if (pathname === "/login") return;

        // If auth is enabled but not configured (no users), redirect to login for setup
        if (!authConfigured) {
            navigate("/login?setup=true");
            return;
        }

        // If auth is enabled and not authenticated, redirect to login
        if (!isAuthenticated) {
            navigate("/login");
            return;
        }
    }, [isLoading, isAuthenticated, authEnabled, authConfigured, pathname, navigate]);

    // Show nothing while loading to prevent flash
    if (isLoading) {
        return (
            <div className="flex items-center justify-center min-h-screen bg-background">
                <div className="flex flex-col items-center gap-4">
                    <div className="w-8 h-8 border-4 border-primary border-t-transparent rounded-full animate-spin" />
                    <p className="text-muted-foreground text-sm">Loading...</p>
                </div>
            </div>
        );
    }

    // If auth is enabled and user is not authenticated (and not on login page)
    if (authEnabled && !isAuthenticated && pathname !== "/login") {
        return null; // Will redirect via useEffect
    }

    return <>{children}</>;
}
