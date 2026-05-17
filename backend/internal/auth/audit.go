package auth

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/apotech/backend/internal/model"
)

// NewAuditInterceptor returns an interceptor that records every NON-read RPC
// to the audit_log table after it completes. Reads (List*, Get*, Search*, Me,
// Ping, GetTodaySnapshot, etc.) are skipped to keep the table actionable.
//
// The interceptor must run AFTER NewInterceptor (auth gate) so the Principal
// is already in context. Order in main.go: WithInterceptors(authInterceptor,
// auditInterceptor).
func NewAuditInterceptor(db *gorm.DB) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			res, err := next(ctx, req)

			procedure := req.Spec().Procedure
			if isReadOnlyProcedure(procedure) {
				return res, err
			}

			entry := model.AuditEntry{
				Procedure:  procedure,
				IP:         req.Peer().Addr,
				UserAgent:  req.Header().Get("User-Agent"),
				DurationMS: int32(time.Since(start).Milliseconds()),
				OK:         err == nil,
			}
			if p, ok := PrincipalFromContext(ctx); ok {
				userID := p.UserID
				entry.UserID = &userID
				entry.Role = p.Role
				if p.BranchID != "" {
					bid := p.BranchID
					entry.BranchID = &bid
				}
			}
			if err != nil {
				if ce := new(connect.Error); connectAs(err, &ce) {
					entry.Code = ce.Code().String()
					entry.Message = ce.Message()
				} else {
					entry.Code = "unknown"
					entry.Message = err.Error()
				}
			}

			// Write asynchronously so audit overhead never blocks the user. Use a
			// detached context so a cancelled caller doesn't lose the audit row.
			go func(e model.AuditEntry) {
				_ = db.WithContext(context.Background()).Create(&e).Error
			}(entry)

			return res, err
		}
	}
}

// isReadOnlyProcedure heuristically detects read-only RPCs by name. The audit
// log is for "what did somebody change" — flooding it with every list/get
// would bury that signal.
func isReadOnlyProcedure(procedure string) bool {
	// procedure looks like "/<package>.<Service>/<Method>"
	idx := strings.LastIndex(procedure, "/")
	if idx < 0 {
		return false
	}
	method := procedure[idx+1:]
	switch {
	case strings.HasPrefix(method, "List"),
		strings.HasPrefix(method, "Get"),
		strings.HasPrefix(method, "Search"),
		method == "Me",
		method == "Ping":
		return true
	}
	return false
}

// connectAs is a tiny wrapper so we don't import errors just for this.
func connectAs(err error, target **connect.Error) bool {
	for cur := err; cur != nil; {
		if e, ok := cur.(*connect.Error); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := cur.(unwrapper); ok {
			cur = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
