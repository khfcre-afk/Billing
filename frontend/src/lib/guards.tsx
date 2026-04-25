import { useEffect, type ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "./auth";

export function RequireAuth({ children }: { children: ReactNode }) {
  const { me, loading, bootstrap } = useAuth();
  const location = useLocation();

  useEffect(() => {
    if (!me && loading) void bootstrap();
  }, [me, loading, bootstrap]);

  if (loading) return <div className="container py-20 text-muted">Loading…</div>;
  if (!me) return <Navigate to="/login" state={{ from: location }} replace />;
  return <>{children}</>;
}

export function RequireAdmin({ children }: { children: ReactNode }) {
  const { me } = useAuth();
  if (me?.role !== "admin") return <Navigate to="/dashboard" replace />;
  return <>{children}</>;
}
