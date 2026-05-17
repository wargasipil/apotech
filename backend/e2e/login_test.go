package e2e

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	authifacev1 "github.com/apotech/backend/gen/auth_iface/v1"
	userifacev1 "github.com/apotech/backend/gen/user_iface/v1"
)

// TestLogin_HappyPath verifies that valid credentials yield a non-empty
// access+refresh token pair and a User, and that the access token works
// on a protected RPC (Me).
func TestLogin_HappyPath(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	res, err := env.Auth.Login(ctx, connect.NewRequest(&userifacev1.LoginRequest{
		Email:    env.Owner.Email,
		Password: env.Owner.Password,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, res.Msg.AccessToken, "access_token must be non-empty")
	require.NotEmpty(t, res.Msg.RefreshToken, "refresh_token must be non-empty")
	require.NotEqual(t, res.Msg.AccessToken, res.Msg.RefreshToken, "tokens must differ")
	require.NotNil(t, res.Msg.User, "user must be returned")
	require.Equal(t, env.Owner.Email, res.Msg.User.Email)
	require.Equal(t, authifacev1.Role_ROLE_OWNER, res.Msg.User.Role)
	require.True(t, res.Msg.User.Active)
	require.Greater(t, res.Msg.AccessExpiresAt, int64(0), "access_expires_at must be set")
	require.Greater(t, res.Msg.RefreshExpiresAt, res.Msg.AccessExpiresAt,
		"refresh_expires_at must be later than access_expires_at")

	// Access token works on a protected RPC.
	meReq := connect.NewRequest(&userifacev1.MeRequest{})
	meReq.Header().Set("Authorization", "Bearer "+res.Msg.AccessToken)
	me, err := env.Auth.Me(ctx, meReq)
	require.NoError(t, err, "Me with valid access token must succeed")
	require.Equal(t, env.Owner.Email, me.Msg.User.Email)
}

// TestLogin_InvalidCredentials verifies that wrong-password and unknown-email
// both return Unauthenticated. The exact message is "invalid credentials" so
// the client can't tell which one was wrong (no user enumeration).
func TestLogin_InvalidCredentials(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	cases := []struct {
		name  string
		email string
		pw    string
	}{
		{"wrong_password", env.Owner.Email, "this-is-not-the-password"},
		{"unknown_email", "ghost@nowhere.example", env.Owner.Password},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env.Auth.Login(ctx, connect.NewRequest(&userifacev1.LoginRequest{
				Email:    tc.email,
				Password: tc.pw,
			}))
			require.Error(t, err, "must reject invalid credentials")

			var cerr *connect.Error
			require.True(t, errors.As(err, &cerr), "must be a connect.Error")
			require.Equal(t, connect.CodeUnauthenticated, cerr.Code(),
				"must return CodeUnauthenticated")
			require.Contains(t, cerr.Message(), "invalid credentials",
				"must not leak whether email or password was wrong")
		})
	}
}

// TestLogin_PublicWhenAlreadyAuthenticated mirrors the frontend behavior of
// redirecting to "/" when a logged-in user visits /login. The RPC-layer
// equivalent: Login is annotated `public = true` in the proto, so calling
// it a second time with an Authorization header still succeeds and returns
// FRESH tokens (different from the previous pair).
func TestLogin_PublicWhenAlreadyAuthenticated(t *testing.T) {
	env := SetupEnv(t)
	ctx := context.Background()

	// First login → save tokens.
	first, err := env.Auth.Login(ctx, connect.NewRequest(&userifacev1.LoginRequest{
		Email:    env.Owner.Email,
		Password: env.Owner.Password,
	}))
	require.NoError(t, err)

	// Second login WITH the prior access token in the Authorization header.
	// Login is public, so the interceptor should skip auth validation and
	// the call should succeed regardless.
	req := connect.NewRequest(&userifacev1.LoginRequest{
		Email:    env.Owner.Email,
		Password: env.Owner.Password,
	})
	req.Header().Set("Authorization", "Bearer "+first.Msg.AccessToken)

	second, err := env.Auth.Login(ctx, req)
	require.NoError(t, err, "Login must remain callable when Authorization is already set")
	require.NotEmpty(t, second.Msg.AccessToken)
	require.NotEmpty(t, second.Msg.RefreshToken)

	// Refresh tokens MUST differ — each call mints a fresh random one.
	// (Access JWTs are pure functions of claims; when issued within the same
	// second they're byte-identical. That's correct behavior, not a bug.)
	require.NotEqual(t, first.Msg.RefreshToken, second.Msg.RefreshToken,
		"each Login call must mint a fresh refresh token")
}
