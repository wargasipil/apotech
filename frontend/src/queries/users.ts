import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { userClient } from "../lib/clients";
import type {
  ChangePasswordRequest,
  CreateUserRequest,
  SetUserActiveRequest,
  UpdateUserRoleRequest,
} from "../gen/user_iface/v1/users_pb";

export const userKeys = {
  all: ["users"] as const,
  list: () => [...userKeys.all, "list"] as const,
};

export function useUsersQuery() {
  return useQuery({
    queryKey: userKeys.list(),
    queryFn: async () => {
      const res = await userClient.listUsers({});
      return res.users;
    },
  });
}

export function useCreateUserMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateUserRequest>) => userClient.createUser(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: userKeys.list() }),
  });
}

export function useUpdateUserRoleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateUserRoleRequest>) =>
      userClient.updateUserRole(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: userKeys.list() }),
  });
}

export function useSetUserActiveMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetUserActiveRequest>) =>
      userClient.setUserActive(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: userKeys.list() }),
  });
}

export function useChangePasswordMutation() {
  return useMutation({
    mutationFn: (req: PartialMessage<ChangePasswordRequest>) =>
      userClient.changePassword(req),
  });
}

// Imperative search helper — backend ILIKEs email/name. Drives the warehouse
// detail "Add user" picker via SearchableSelect's loadOptions async mode.
export async function searchUsers(query: string) {
  const res = await userClient.searchUsers({ query, limit: 20 });
  return [...res.users];
}
