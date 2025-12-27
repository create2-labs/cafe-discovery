package service

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/pqc"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserAlreadyExists = errors.New("user already exists")
)

var b64u = base64.RawURLEncoding

const (
	MLDSA_ALG = "ML-DSA-65"
	EDDSA_ALG = "EdDSA"
)

func b64uEncode(b []byte) string { return b64u.EncodeToString(b) }
func b64uDecode(s string) ([]byte, error) {
	return b64u.DecodeString(s)
}

// serverKeys holds the cryptographic keys for JWT signing
type serverKeys struct {
	edPriv ed25519.PrivateKey
	edPub  ed25519.PublicKey
	pqc    *pqc.MLDSA
}

func newServerKeys() (*serverKeys, error) {
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	// ML-DSA-65 is a standard parameter set in liboqs.
	// We'll use a context string to domain-separate signatures (used when supported by the scheme).
	pqc, err := pqc.NewMLDSA(MLDSA_ALG, []byte("JWT"))
	if err != nil {
		return nil, err
	}

	return &serverKeys{edPriv: edPriv, edPub: edPub, pqc: pqc}, nil
}

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// AuthService handles authentication operations
// Hybrid PQC tokens (EdDSA + ML-DSA-65) are supported
type AuthService struct {
	userRepo  repository.UserRepository
	planRepo  repository.PlanRepository
	jwtSecret []byte
	jwtExpiry time.Duration
	keys      *serverKeys
}

// NewAuthService creates a new auth service with hybrid PQC support
// Uses hybrid tokens (EdDSA + ML-DSA-65)
func NewAuthService(userRepo repository.UserRepository, planRepo repository.PlanRepository, jwtSecret string, jwtExpiry time.Duration) (*AuthService, error) {
	// Initialize PQC keys, required for hybrid mode
	keys, err := newServerKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PQC keys: %w", err)
	}

	return &AuthService{
		userRepo:  userRepo,
		planRepo:  planRepo,
		jwtSecret: []byte(jwtSecret), // Kept for API compatibility but not used for signing
		jwtExpiry: jwtExpiry,
		keys:      keys,
	}, nil
}

// Close releases resources associated with the auth service
func (s *AuthService) Close() {
	if s.keys != nil && s.keys.pqc != nil {
		s.keys.pqc.Close()
	}
}

// SignupRequest represents the signup request
type SignupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// SigninRequest represents the signin request
type SigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Token string       `json:"token"`
	User  *domain.User `json:"user"`
}

// Signup creates a new user account
func (s *AuthService) Signup(req SignupRequest) (*AuthResponse, error) {
	// Validate passwords match
	if req.Password != req.ConfirmPassword {
		return nil, fmt.Errorf("passwords do not match")
	}

	// Validate email and password
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(req.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	// Check if user already exists
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Get FREE plan
	freePlan, err := s.planRepo.FindByType(domain.PlanTypeFree)
	if err != nil {
		return nil, fmt.Errorf("failed to get free plan: %w", err)
	}
	if freePlan == nil {
		return nil, fmt.Errorf("free plan not found - please run migrations")
	}

	// Create user with FREE plan
	user := &domain.User{
		Email:    req.Email,
		Password: string(hashedPassword),
		PlanID:   freePlan.ID,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	token, err := s.generateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// Signin authenticates a user and returns a JWT token
func (s *AuthService) Signin(req SigninRequest) (*AuthResponse, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Find user by email
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidPassword
	}

	// Generate JWT token
	token, err := s.generateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// ValidateToken validates a JWT token and returns the claims
// Only hybrid PQC tokens (EdDSA + ML-DSA-65) are accepted
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	return s.verifyHybridJWT(tokenString)
}

// generateToken generates a hybrid PQC JWT token for the user
// Always uses hybrid signatures (EdDSA + ML-DSA-65)
func (s *AuthService) generateToken(userID uuid.UUID, email string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	return s.mintHybridJWT(claims)
}

// JWS structures for hybrid tokens
type jwsHeader struct {
	Alg  string   `json:"alg"`
	Typ  string   `json:"typ,omitempty"`
	Kid  string   `json:"kid,omitempty"`
	Crit []string `json:"crit,omitempty"`
}

type jwsJSON struct {
	Payload    string         `json:"payload"`
	Signatures []jwsSignature `json:"signatures"`
}

type jwsSignature struct {
	Protected string `json:"protected"`
	Signature string `json:"signature"`
}

func signingInput(protectedB64, payloadB64 string) []byte {
	return []byte(protectedB64 + "." + payloadB64)
}

// mintHybridJWT generates a hybrid JWT token (EdDSA + ML-DSA)
func (s *AuthService) mintHybridJWT(claims *JWTClaims) (string, error) {
	if s.keys == nil {
		return "", fmt.Errorf("PQC keys not initialized")
	}

	payload, _ := json.Marshal(claims)
	payloadB64 := b64uEncode(payload)

	// EdDSA signature
	h1 := jwsHeader{Alg: EDDSA_ALG, Typ: "JWT", Kid: "ed25519-cafe-1"}
	h1b, _ := json.Marshal(h1)
	h1b64 := b64uEncode(h1b)
	in1 := signingInput(h1b64, payloadB64)
	sig1 := ed25519.Sign(s.keys.edPriv, in1)

	// ML-DSA signature via liboqs
	h2 := jwsHeader{Alg: MLDSA_ALG, Typ: "JWT", Kid: "mldsa65-cafe-1", Crit: []string{"alg"}}
	h2b, _ := json.Marshal(h2)
	h2b64 := b64uEncode(h2b)
	in2 := signingInput(h2b64, payloadB64)

	sig2, err := s.keys.pqc.Sign(in2)
	if err != nil {
		return "", err
	}

	obj := jwsJSON{
		Payload: payloadB64,
		Signatures: []jwsSignature{
			{Protected: h1b64, Signature: b64uEncode(sig1)},
			{Protected: h2b64, Signature: b64uEncode(sig2)},
		},
	}
	raw, _ := json.Marshal(obj)
	return b64uEncode(raw), nil
}

// parseHybridJWS parses a hybrid JWS token
func parseHybridJWS(token string) (*jwsJSON, error) {
	raw, err := b64uDecode(token)
	if err != nil {
		return nil, fmt.Errorf("token decode: %w", err)
	}
	var obj jwsJSON
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("token json: %w", err)
	}
	if obj.Payload == "" || len(obj.Signatures) < 2 {
		return nil, fmt.Errorf("missing payload or signatures")
	}
	return &obj, nil
}

// parseAndValidateClaims parses and validates JWT claims
func parseAndValidateClaims(payloadB64 string) (*JWTClaims, error) {
	payloadBytes, err := b64uDecode(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("payload decode: %w", err)
	}
	var c JWTClaims
	if err := json.Unmarshal(payloadBytes, &c); err != nil {
		return nil, fmt.Errorf("claims: %w", err)
	}
	// Validate expiration
	if c.ExpiresAt != nil && time.Now().After(c.ExpiresAt.Time) {
		return nil, fmt.Errorf("token expired")
	}
	return &c, nil
}

// verifySignature verifies a single signature in a hybrid JWS
func (s *AuthService) verifySignature(sig jwsSignature, payloadB64 string) (edOK, pqcOK bool, err error) {
	hb, err := b64uDecode(sig.Protected)
	if err != nil {
		return false, false, fmt.Errorf("protected decode: %w", err)
	}
	var h jwsHeader
	if err := json.Unmarshal(hb, &h); err != nil {
		return false, false, fmt.Errorf("header parse: %w", err)
	}
	sigBytes, err := b64uDecode(sig.Signature)
	if err != nil {
		return false, false, fmt.Errorf("sig decode: %w", err)
	}
	input := signingInput(sig.Protected, payloadB64)

	switch h.Alg {
	case EDDSA_ALG:
		if ed25519.Verify(s.keys.edPub, input, sigBytes) {
			edOK = true
		}
	case MLDSA_ALG:
		ok, err := s.keys.pqc.Verify(input, sigBytes)
		if err != nil {
			return false, false, err
		}
		if ok {
			pqcOK = true
		}
	}
	return edOK, pqcOK, nil
}

// verifyHybridJWT verifies a hybrid JWT token
func (s *AuthService) verifyHybridJWT(token string) (*JWTClaims, error) {
	if s.keys == nil {
		return nil, fmt.Errorf("PQC keys not initialized")
	}

	obj, err := parseHybridJWS(token)
	if err != nil {
		return nil, err
	}

	c, err := parseAndValidateClaims(obj.Payload)
	if err != nil {
		return nil, err
	}

	var edOK, pqcOK bool
	for _, sig := range obj.Signatures {
		edVerified, pqcVerified, err := s.verifySignature(sig, obj.Payload)
		if err != nil {
			return nil, err
		}
		if edVerified {
			edOK = true
		}
		if pqcVerified {
			pqcOK = true
		}
	}

	if !edOK || !pqcOK {
		return nil, fmt.Errorf("hybrid signature invalid (ed=%v pqc=%v)", edOK, pqcOK)
	}
	return c, nil
}
