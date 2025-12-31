"use client";

import { createContext, useContext, useEffect, useState, useCallback } from "react";
import { api, User, AuthStatus } from "@/lib/api";

interface AuthContextType {
    user: User | null;
    isLoading: boolean;
    isAuthenticated: boolean;
    authEnabled: boolean;
    authConfigured: boolean;
    login: (username: string, password: string) => Promise<void>;
    logout: () => Promise<void>;
    register: (username: string, password: string) => Promise<void>;
    refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [authStatus, setAuthStatus] = useState<AuthStatus | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    const refreshUser = useCallback(async () => {
        try {
            const currentUser = await api.getCurrentUser();
            setUser(currentUser);
        } catch {
            setUser(null);
        }
    }, []);

    useEffect(() => {
        let mounted = true;

        const initialize = async () => {
            try {
                // Get auth status
                const status = await api.authStatus();

                if (!mounted) return;

                // If auth is enabled, try to get current user in parallel
                if (status.enabled) {
                    try {
                        const currentUser = await api.getCurrentUser();
                        if (mounted) {
                            // Batch all state updates together
                            setAuthStatus(status);
                            setUser(currentUser);
                            setIsLoading(false);
                        }
                    } catch {
                        if (mounted) {
                            setAuthStatus(status);
                            setUser(null);
                            setIsLoading(false);
                        }
                    }
                } else {
                    if (mounted) {
                        setAuthStatus(status);
                        setIsLoading(false);
                    }
                }
            } catch (error) {
                console.error("Failed to initialize auth:", error);
                if (mounted) {
                    setIsLoading(false);
                }
            }
        };

        initialize();

        return () => {
            mounted = false;
        };
    }, []);

    const login = useCallback(async (username: string, password: string) => {
        const response = await api.login({ username, password });
        setUser({
            id: response.id,
            username: response.username,
            createdAt: response.createdAt,
        });
    }, []);

    const logout = useCallback(async () => {
        await api.logout();
        setUser(null);
    }, []);

    const register = useCallback(async (username: string, password: string) => {
        await api.register({ username, password });
        // After registration, automatically log in
        await login(username, password);
    }, [login]);

    const value: AuthContextType = {
        user,
        isLoading,
        isAuthenticated: !!user,
        authEnabled: authStatus?.enabled ?? false,
        authConfigured: authStatus?.configured ?? false,
        login,
        logout,
        register,
        refreshUser,
    };

    return (
        <AuthContext.Provider value={value}>
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error("useAuth must be used within an AuthProvider");
    }
    return context;
}
