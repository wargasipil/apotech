package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Issuer struct {
	Secret []byte
	TTL    time.Duration
}

func (i *Issuer) Issue(userID, role string) (string, time.Time, error) {
	exp := time.Now().Add(i.TTL)
	c := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	s, err := tok.SignedString(i.Secret)
	return s, exp, err
}

func (i *Issuer) Parse(token string) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return i.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}
