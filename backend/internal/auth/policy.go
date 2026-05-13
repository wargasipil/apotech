package auth

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	authifacev1 "github.com/apotech/backend/gen/auth_iface/v1"
)

// Policy describes the auth requirement for a single RPC procedure.
//   - Public=true: skip authn entirely.
//   - Public=false + AllowedRoles empty: authn required, no role check.
//   - Public=false + AllowedRoles non-empty: authn required AND caller.Role must be a key.
type Policy struct {
	Public       bool
	AllowedRoles map[string]struct{}
}

// BuildPolicy walks every registered proto file and builds a map keyed by
// Connect procedure path (e.g. "/user_iface.v1.UserService/CreateUser"). Reads
// the per-rpc options declared in auth_iface/v1/policy.proto.
func BuildPolicy() map[string]Policy {
	out := map[string]Policy{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Services().Len(); i++ {
			sd := fd.Services().Get(i)
			for j := 0; j < sd.Methods().Len(); j++ {
				md := sd.Methods().Get(j)
				proc := "/" + string(sd.FullName()) + "/" + string(md.Name())
				p := Policy{AllowedRoles: map[string]struct{}{}}

				opts := md.Options()
				if public, ok := proto.GetExtension(opts, authifacev1.E_Public).(bool); ok && public {
					p.Public = true
				}
				if roles, ok := proto.GetExtension(opts, authifacev1.E_AllowedRoles).([]authifacev1.Role); ok {
					for _, r := range roles {
						if s := roleEnumToString(r); s != "" {
							p.AllowedRoles[s] = struct{}{}
						}
					}
				}
				out[proc] = p
			}
		}
		return true
	})
	return out
}

// roleEnumToString maps the proto enum to the bare role string we store in the
// DB and embed in JWT claims (matches model.User.Role).
func roleEnumToString(r authifacev1.Role) string {
	switch r {
	case authifacev1.Role_ROLE_OWNER:
		return "OWNER"
	case authifacev1.Role_ROLE_PHARMACIST:
		return "PHARMACIST"
	case authifacev1.Role_ROLE_CASHIER:
		return "CASHIER"
	default:
		return ""
	}
}
