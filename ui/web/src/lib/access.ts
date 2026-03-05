export type UserRole = "admin" | "operator" | "viewer";

const roleRank: Record<UserRole, number> = {
  viewer: 1,
  operator: 2,
  admin: 3,
};

export function hasRole(role: UserRole, required: UserRole): boolean {
  return roleRank[role] >= roleRank[required];
}
