package auth

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
)

type Principal struct {
	UserID      string
	Role        string
	BranchID    string // deprecated; superseded by WarehouseID
	WarehouseID string // active warehouse, parsed from the X-Warehouse-Id request header
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

func MustPrincipal(ctx context.Context) (Principal, error) {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}
	return p, nil
}
