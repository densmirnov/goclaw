import { Navigate } from "react-router";
import { useAuthStore } from "@/stores/use-auth-store";
import { hasRole, type UserRole } from "@/lib/access";
import { ROUTES } from "@/lib/constants";

export function RequireRole({
  minRole,
  children,
}: {
  minRole: UserRole;
  children: React.ReactNode;
}) {
  const role = useAuthStore((s) => s.role);
  const connected = useAuthStore((s) => s.connected);

  if (!connected) {
    return null;
  }

  if (!hasRole(role, minRole)) {
    return <Navigate to={ROUTES.OVERVIEW} replace />;
  }

  return <>{children}</>;
}
