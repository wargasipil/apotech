package e2e

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	unitifacev1 "github.com/apotech/backend/gen/unit_iface/v1"
)

// TestUnitCatalog_CRUD covers the global unit catalog: create base, attach two
// derivatives, list returns the hydrated tree, archive a derivative, archive
// the base (CASCADE leaves no orphans).
func TestUnitCatalog_CRUD(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	// Use a unique base name per run so the suite can be re-run against the dev DB.
	baseName := fmt.Sprintf("tablet-crud-%d", time.Now().UnixNano())
	createBase, err := env.Units.CreateUnitBase(ctx, authReq(env, t,
		&unitifacev1.CreateUnitBaseRequest{Name: baseName}))
	require.NoError(t, err)
	baseID := createBase.Msg.Base.Id
	require.NotEmpty(t, baseID)
	require.True(t, createBase.Msg.Base.Active)
	t.Cleanup(func() {
		_, _ = env.Units.ArchiveUnitBase(ctx, authReq(env, t,
			&unitifacev1.ArchiveUnitBaseRequest{Id: baseID}))
	})

	// Two derivatives: strip ×10, box ×100.
	stripRes, err := env.Units.CreateUnitDerivative(ctx, authReq(env, t,
		&unitifacev1.CreateUnitDerivativeRequest{
			BaseUnitId: baseID, Name: "strip", Factor: 10, SortOrder: 1,
		}))
	require.NoError(t, err)
	stripID := stripRes.Msg.Derivative.Id

	boxRes, err := env.Units.CreateUnitDerivative(ctx, authReq(env, t,
		&unitifacev1.CreateUnitDerivativeRequest{
			BaseUnitId: baseID, Name: "box", Factor: 100, SortOrder: 2,
		}))
	require.NoError(t, err)
	require.Equal(t, int64(100), boxRes.Msg.Derivative.Factor)

	// List returns the base with both derivatives hydrated.
	list, err := env.Units.ListUnitBases(ctx, authReq(env, t,
		&unitifacev1.ListUnitBasesRequest{}))
	require.NoError(t, err)
	var got *unitifacev1.UnitBase
	for _, b := range list.Msg.Bases {
		if b.Id == baseID {
			got = b
			break
		}
	}
	require.NotNil(t, got, "newly created base must be in the list")
	names := []string{}
	for _, d := range got.Derivatives {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"strip", "box"}, names, "derivatives ordered by sort_order")

	// Archive the strip derivative → list omits it.
	_, err = env.Units.ArchiveUnitDerivative(ctx, authReq(env, t,
		&unitifacev1.ArchiveUnitDerivativeRequest{Id: stripID}))
	require.NoError(t, err)
	list, err = env.Units.ListUnitBases(ctx, authReq(env, t,
		&unitifacev1.ListUnitBasesRequest{}))
	require.NoError(t, err)
	for _, b := range list.Msg.Bases {
		if b.Id != baseID {
			continue
		}
		for _, d := range b.Derivatives {
			require.NotEqual(t, "strip", d.Name, "archived derivative must not appear")
		}
	}

	// Archive the base → with include_inactive=true the base reappears as inactive.
	_, err = env.Units.ArchiveUnitBase(ctx, authReq(env, t,
		&unitifacev1.ArchiveUnitBaseRequest{Id: baseID}))
	require.NoError(t, err)
	withInactive, err := env.Units.ListUnitBases(ctx, authReq(env, t,
		&unitifacev1.ListUnitBasesRequest{IncludeInactive: true}))
	require.NoError(t, err)
	found := false
	for _, b := range withInactive.Msg.Bases {
		if b.Id == baseID {
			require.False(t, b.Active)
			found = true
		}
	}
	require.True(t, found, "archived base should reappear with include_inactive")
}

// TestUnitCatalog_GuardsFactor rejects factor <= 1.
func TestUnitCatalog_GuardsFactor(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	base, err := env.Units.CreateUnitBase(ctx, authReq(env, t,
		&unitifacev1.CreateUnitBaseRequest{
			Name: fmt.Sprintf("ml-guard-%d", time.Now().UnixNano()),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Units.ArchiveUnitBase(ctx, authReq(env, t,
			&unitifacev1.ArchiveUnitBaseRequest{Id: base.Msg.Base.Id}))
	})

	for _, bad := range []int64{0, 1, -5} {
		_, err := env.Units.CreateUnitDerivative(ctx, authReq(env, t,
			&unitifacev1.CreateUnitDerivativeRequest{
				BaseUnitId: base.Msg.Base.Id, Name: "x", Factor: bad,
			}))
		require.Error(t, err, "factor %d must be rejected", bad)
		var cerr *connect.Error
		require.True(t, errors.As(err, &cerr))
		require.Equal(t, connect.CodeInvalidArgument, cerr.Code())
	}
}

// TestUnitCatalog_DuplicateDerivative: same (base, name) twice while active →
// AlreadyExists (the partial unique index fires).
func TestUnitCatalog_DuplicateDerivative(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()
	base, err := env.Units.CreateUnitBase(ctx, authReq(env, t,
		&unitifacev1.CreateUnitBaseRequest{
			Name: fmt.Sprintf("ml-dup-%d", time.Now().UnixNano()),
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.Units.ArchiveUnitBase(ctx, authReq(env, t,
			&unitifacev1.ArchiveUnitBaseRequest{Id: base.Msg.Base.Id}))
	})

	_, err = env.Units.CreateUnitDerivative(ctx, authReq(env, t,
		&unitifacev1.CreateUnitDerivativeRequest{
			BaseUnitId: base.Msg.Base.Id, Name: "liter", Factor: 1000,
		}))
	require.NoError(t, err)
	_, err = env.Units.CreateUnitDerivative(ctx, authReq(env, t,
		&unitifacev1.CreateUnitDerivativeRequest{
			BaseUnitId: base.Msg.Base.Id, Name: "liter", Factor: 1000,
		}))
	require.Error(t, err)
	var cerr *connect.Error
	require.True(t, errors.As(err, &cerr))
	require.Equal(t, connect.CodeAlreadyExists, cerr.Code())
}
