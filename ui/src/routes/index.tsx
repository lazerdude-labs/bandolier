import { useEffect } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { getAuthStatus } from '@/lib/api';

export function IndexPage() {
  const nav = useNavigate();
  const { data, isLoading } = useQuery({
    queryKey: ['auth-status'],
    queryFn: getAuthStatus,
    retry: false,
  });

  useEffect(() => {
    if (isLoading) return;
    if (data?.configured === false) {
      nav({ to: '/setup', replace: true });
    } else {
      nav({ to: '/clusters', replace: true });
    }
  }, [data, isLoading, nav]);

  return null;
}
