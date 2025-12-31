"use client";

import { useState, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "@/components/AuthProvider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Boxes, LogIn, UserPlus, AlertCircle, Loader2 } from "lucide-react";

export default function Login() {
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const { login, register, isAuthenticated, authEnabled, authConfigured, isLoading } = useAuth();

    const [isSetupMode, setIsSetupMode] = useState(false);
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [error, setError] = useState("");
    const [submitting, setSubmitting] = useState(false);

    // Handle setup mode and navigation in a single effect to prevent flicker
    useEffect(() => {
        // Determine setup mode
        const setupParam = searchParams.get("setup");
        if (setupParam === "true" || !authConfigured) {
            setIsSetupMode(true);
        }

        // Handle navigation after auth check complete
        if (!isLoading) {
            if (isAuthenticated) {
                navigate("/", { replace: true });
            } else if (!authEnabled) {
                navigate("/", { replace: true });
            }
        }
    }, [searchParams, authConfigured, isLoading, isAuthenticated, authEnabled, navigate]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        setSubmitting(true);

        try {
            if (isSetupMode) {
                if (password !== confirmPassword) {
                    setError("Passwords do not match");
                    setSubmitting(false);
                    return;
                }
                if (password.length < 8) {
                    setError("Password must be at least 8 characters");
                    setSubmitting(false);
                    return;
                }
                await register(username, password);
            } else {
                await login(username, password);
            }
            navigate("/");
        } catch (err) {
            setError(err instanceof Error ? err.message : "An error occurred");
        } finally {
            setSubmitting(false);
        }
    };

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

    return (
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background via-background to-primary/5 p-4">
            <Card className="w-full max-w-md shadow-xl border-border/50 backdrop-blur">
                <CardHeader className="text-center space-y-4">
                    <div className="flex justify-center">
                        <div className="p-3 rounded-xl bg-gradient-to-br from-primary/20 to-purple-500/20">
                            <Boxes className="w-10 h-10 text-primary" />
                        </div>
                    </div>
                    <div>
                        <CardTitle className="text-2xl font-bold">
                            {isSetupMode ? "Welcome to DBnest" : "Sign In"}
                        </CardTitle>
                        <CardDescription className="mt-2">
                            {isSetupMode
                                ? "Create your admin account to get started"
                                : "Enter your credentials to access DBnest"
                            }
                        </CardDescription>
                    </div>
                </CardHeader>
                <CardContent>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        {error && (
                            <div className="flex items-center gap-2 p-3 rounded-lg bg-destructive/10 text-destructive text-sm">
                                <AlertCircle className="w-4 h-4 shrink-0" />
                                <span>{error}</span>
                            </div>
                        )}

                        <div className="space-y-2">
                            <Label htmlFor="username">Username</Label>
                            <Input
                                id="username"
                                type="text"
                                placeholder="Enter your username"
                                value={username}
                                onChange={(e) => setUsername(e.target.value)}
                                required
                                autoComplete="username"
                                autoFocus
                            />
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="password">Password</Label>
                            <Input
                                id="password"
                                type="password"
                                placeholder="Enter your password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                required
                                autoComplete={isSetupMode ? "new-password" : "current-password"}
                            />
                        </div>

                        {isSetupMode && (
                            <div className="space-y-2">
                                <Label htmlFor="confirmPassword">Confirm Password</Label>
                                <Input
                                    id="confirmPassword"
                                    type="password"
                                    placeholder="Confirm your password"
                                    value={confirmPassword}
                                    onChange={(e) => setConfirmPassword(e.target.value)}
                                    required
                                    autoComplete="new-password"
                                />
                            </div>
                        )}

                        <Button
                            type="submit"
                            className="w-full"
                            disabled={submitting}
                        >
                            {submitting ? (
                                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                            ) : isSetupMode ? (
                                <UserPlus className="w-4 h-4 mr-2" />
                            ) : (
                                <LogIn className="w-4 h-4 mr-2" />
                            )}
                            {submitting
                                ? "Please wait..."
                                : isSetupMode
                                    ? "Create Account"
                                    : "Sign In"
                            }
                        </Button>
                    </form>

                    {isSetupMode && (
                        <p className="mt-4 text-center text-xs text-muted-foreground">
                            This will create the first admin account for your DBnest instance.
                        </p>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
