import { useState, useCallback, useEffect } from "react";
import { Sidebar } from "@/components/Sidebar";
import { CreateDatabaseModal } from "@/components/CreateDatabaseModal";
import { NetworkTopology } from "@/components/NetworkTopology";
import { Button } from "@/components/ui/button";
import { RefreshCw } from "lucide-react";
import { api, TopologyNetwork } from "@/lib/api";

export default function Topology() {
    const [createModalOpen, setCreateModalOpen] = useState(false);
    const [topology, setTopology] = useState<TopologyNetwork[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [isRefreshing, setIsRefreshing] = useState(false);

    const fetchTopology = useCallback(async () => {
        try {
            setError(null);
            const data = await api.getTopology();
            setTopology(data);
        } catch (err) {
            console.error('Failed to fetch topology:', err);
            setError('Failed to load network topology');
        } finally {
            setLoading(false);
            setIsRefreshing(false);
        }
    }, []);

    useEffect(() => {
        fetchTopology();

        const handleVisibilityChange = () => {
            if (document.visibilityState === 'visible') {
                fetchTopology();
            }
        };
        document.addEventListener('visibilitychange', handleVisibilityChange);
        return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
    }, [fetchTopology]);

    const handleRefresh = async () => {
        setIsRefreshing(true);
        await fetchTopology();
    };

    return (
        <div className="flex min-h-screen bg-background">
            <Sidebar onCreateDatabase={() => setCreateModalOpen(true)} />

            <main className="flex-1 overflow-auto">
                <header className="sticky top-0 z-10 bg-background/80 backdrop-blur-sm border-b border-border">
                    <div className="px-6 py-4 flex items-center justify-between">
                        <div>
                            <h1 className="text-2xl font-bold">Network Topology</h1>
                            <p className="text-muted-foreground text-sm">
                                Visualize your database network
                            </p>
                        </div>
                        <Button variant="outline" size="sm" onClick={handleRefresh} disabled={isRefreshing}>
                            <RefreshCw className={`w-4 h-4 mr-2 ${isRefreshing ? 'animate-spin' : ''}`} />
                            Refresh
                        </Button>
                    </div>
                </header>

                <div className="p-6">
                    <NetworkTopology topology={topology} loading={loading} error={error} />
                </div>
            </main>

            <CreateDatabaseModal
                open={createModalOpen}
                onOpenChange={setCreateModalOpen}
                onCreate={() => { }}
            />
        </div>
    );
}
