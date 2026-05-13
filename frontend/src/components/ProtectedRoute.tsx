import { Navigate, Outlet } from "react-router-dom";
import { Box, Spinner, Text } from "@chakra-ui/react";

import { useAuth } from "../lib/auth";
import { Role } from "../gen/auth_iface/v1/policy_pb";

type Props = {
  requiredRole?: Role;
  requiredRoles?: Role[];
};

export default function ProtectedRoute({ requiredRole, requiredRoles }: Props) {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  const allowed =
    requiredRoles && requiredRoles.length > 0
      ? requiredRoles.includes(user.role)
      : requiredRole !== undefined
      ? user.role === requiredRole
      : true;

  if (!allowed) {
    return (
      <Box p={8}>
        <Text color="red.500">Access denied: requires a different role.</Text>
      </Box>
    );
  }

  return <Outlet />;
}
