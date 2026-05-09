import { useEffect } from 'react';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { Outlet, useLocation, useMatch, useNavigate } from '@tanstack/react-router';
import { TopBar } from '@/components/TopBar';
import { ToastRegion } from '@/components/ToastRegion';
import { applyInitialTheme } from '@/lib/theme';
import { api, ApiError, listClusters, listProfiles } from '@/lib/api';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

function Shell() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const isAuthRoute = pathname === '/login' || pathname === '/setup';
  const clustersQuery = useQuery({
    queryKey: ['clusters'],
    queryFn: listClusters,
    enabled: !isAuthRoute,
    retry: (failureCount, err: unknown) => {
      if (err instanceof ApiError && err.status === 401) return false;
      return failureCount < 1;
    },
  });
  const profilesQuery = useQuery({
    queryKey: ['profiles'],
    queryFn: listProfiles,
    enabled: !isAuthRoute,
    staleTime: 5 * 60 * 1000, // profiles rarely change
  });

  // 401 → bounce to login
  useEffect(() => {
    if (clustersQuery.error instanceof ApiError && clustersQuery.error.status === 401 && !isAuthRoute) {
      navigate({ to: '/login' });
    }
  }, [clustersQuery.error, isAuthRoute, navigate]);

  const match = useMatch({ from: '/clusters/$clusterId', shouldThrow: false });
  const currentClusterId = match?.params.clusterId;

  if (isAuthRoute) {
    return (
      <>
        <Outlet />
        <ToastRegion />
      </>
    );
  }

  return (
    <>
      <TopBar
        clusters={clustersQuery.data ?? []}
        profiles={profilesQuery.data ?? []}
        currentClusterId={currentClusterId}
        onLogout={async () => {
          await api('POST', '/api/auth/logout');
          navigate({ to: '/login' });
        }}
      />
      <main className="page">
        <Outlet />
      </main>
      <ToastRegion />
    </>
  );
}

export function App() {
  useEffect(() => { applyInitialTheme(); }, []);
  return (
    <QueryClientProvider client={queryClient}>
      <Shell />
    </QueryClientProvider>
  );
}
