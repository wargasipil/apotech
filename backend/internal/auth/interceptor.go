package auth

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
)

// NewInterceptor returns a unary interceptor that enforces the per-RPC policy
// built from proto descriptors (see BuildPolicy). It is the single auth gate:
//   - Public RPCs pass through with no JWT check.
//   - All other RPCs require a valid Bearer JWT.
//   - If the policy lists AllowedRoles, the caller's role must be in that set.
//
// On success the Principal is injected into the request context for handlers
// that need row-level checks (e.g. self-vs-other on ChangePassword).
func NewInterceptor(issuer *Issuer, policy map[string]Policy) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			pol := policy[req.Spec().Procedure]
			if pol.Public {
				return next(ctx, req)
			}

			claims, err := parseBearer(req.Header().Get("Authorization"), issuer)
			if err != nil {
				return nil, err
			}

			if len(pol.AllowedRoles) > 0 {
				if _, ok := pol.AllowedRoles[claims.Role]; !ok {
					return nil, connect.NewError(connect.CodePermissionDenied,
						fmt.Errorf("role %q not permitted", claims.Role))
				}
			}

			ctx = WithPrincipal(ctx, Principal{
				UserID:      claims.UserID,
				Role:        claims.Role,
				BranchID:    req.Header().Get("X-Branch-Id"),
				WarehouseID: req.Header().Get("X-Warehouse-Id"),
			})
			return next(ctx, req)
		}
	}
}

func parseBearer(header string, issuer *Issuer) (*Claims, error) {
	token := strings.TrimPrefix(header, "Bearer ")
	if token == "" || token == header {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing bearer token"))
	}
	claims, err := issuer.Parse(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
	}
	return claims, nil
}
