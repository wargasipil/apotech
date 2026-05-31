package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	authifacev1 "github.com/apotech/backend/gen/auth_iface/v1"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
	warehouseifacev1 "github.com/apotech/backend/gen/warehouse_iface/v1"
)

// TestEnsureBootstrapOwner_GrantsDefaultWarehouse proves that the bootstrap
// owner gets a default-warehouse membership on every boot — closes the gap
// where migration 00019's at-migration grant produced 0 rows on a fresh DB.
func TestEnsureBootstrapOwner_GrantsDefaultWarehouse(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	// After SetupEnv (which already calls EnsureBootstrapOwner internally), the
	// owner must have at least one membership for the global default warehouse.
	mems, err := env.Warehouses.ListUserWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListUserWarehousesRequest{UserId: ""}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(mems.Msg.Memberships), 1,
		"bootstrap owner should have at least one warehouse membership")

	// Find the default warehouse — exactly one warehouse must be global default.
	all, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 1000}))
	require.NoError(t, err)
	var defaultWHID string
	for _, w := range all.Msg.Warehouses {
		if w.IsDefault {
			defaultWHID = w.Id
			break
		}
	}
	require.NotEmpty(t, defaultWHID, "a global default warehouse must exist")

	// The owner's memberships must include the default warehouse.
	hasDefault := false
	for _, m := range mems.Msg.Memberships {
		if m.WarehouseId == defaultWHID {
			hasDefault = true
		}
	}
	require.True(t, hasDefault, "bootstrap owner must have a membership for the default warehouse")
}

// TestCreateUser_GrantsDefaultWarehouse proves that a user created via the
// admin RPC lands with a membership for the global default warehouse, so
// they can use the app immediately without an OWNER having to grant access.
func TestCreateUser_GrantsDefaultWarehouse(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	uniq := time.Now().UnixNano()

	// Owner creates a CASHIER.
	newEmail := fmt.Sprintf("cashier-%d@apotech.local", uniq)
	created, err := env.Users.CreateUser(ctx, authReq(env, t,
		&userifacev1.CreateUserRequest{
			Email:    newEmail,
			Name:     "Test Cashier",
			Password: "Test1234!",
			Role:     authifacev1.Role_ROLE_CASHIER,
		}))
	require.NoError(t, err)
	newUserID := created.Msg.User.Id
	require.NotEmpty(t, newUserID)
	t.Cleanup(func() {
		_, _ = env.Users.SetUserActive(ctx, authReq(env, t,
			&userifacev1.SetUserActiveRequest{UserId: newUserID, Active: false}))
	})

	// The new user must have a membership for the default warehouse.
	mems, err := env.Warehouses.ListUserWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListUserWarehousesRequest{UserId: newUserID}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(mems.Msg.Memberships), 1,
		"new user should have a warehouse membership")

	// Find the default warehouse.
	all, err := env.Warehouses.ListWarehouses(ctx, authReq(env, t,
		&warehouseifacev1.ListWarehousesRequest{IncludeInactive: true, Limit: 1000}))
	require.NoError(t, err)
	var defaultWHID string
	for _, w := range all.Msg.Warehouses {
		if w.IsDefault {
			defaultWHID = w.Id
			break
		}
	}
	require.NotEmpty(t, defaultWHID)

	// Membership for default warehouse exists AND is marked default for this user.
	foundDefault := false
	for _, m := range mems.Msg.Memberships {
		if m.WarehouseId == defaultWHID {
			foundDefault = true
			require.True(t, m.IsDefault,
				"freshly-created user's default-warehouse membership should be is_default=true")
		}
	}
	require.True(t, foundDefault, "new user must have a membership for the default warehouse")
}
