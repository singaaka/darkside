package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	darksidev1 "github.com/singaaka/darkside/gen/go/darkside/v1"
	"github.com/singaaka/darkside/internal/ageenv"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/store"
)

type envHandler struct {
	store *store.Store
}

var envNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,19}$`)

func (h *envHandler) List(ctx context.Context, req *connect.Request[darksidev1.ListEnvironmentsRequest]) (*connect.Response[darksidev1.ListEnvironmentsResponse], error) {
	envs, err := h.store.ListEnvironments(ctx, req.Msg.AppId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*darksidev1.Environment, 0, len(envs))
	for _, e := range envs {
		out = append(out, envToProto(e))
	}
	return connect.NewResponse(&darksidev1.ListEnvironmentsResponse{Environments: out}), nil
}

func (h *envHandler) Get(ctx context.Context, req *connect.Request[darksidev1.GetEnvironmentRequest]) (*connect.Response[darksidev1.GetEnvironmentResponse], error) {
	app, err := h.store.GetApp(ctx, req.Msg.AppId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("app not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	env, err := h.store.GetEnvironment(ctx, dbgen.GetEnvironmentParams{
		AppID: req.Msg.AppId,
		Name:  req.Msg.Name,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("environment not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&darksidev1.GetEnvironmentResponse{
		Environment: envToProto(env),
		Setup:       buildSetup(app.Name, env.Name, env.AgePublicKey, env.AgePrivateKey),
	}), nil
}

func (h *envHandler) Create(ctx context.Context, req *connect.Request[darksidev1.CreateEnvironmentRequest]) (*connect.Response[darksidev1.GetEnvironmentResponse], error) {
	if !envNameRe.MatchString(req.Msg.Name) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must match [a-z][a-z0-9-]{0,19} (got %q)", req.Msg.Name))
	}
	app, err := h.store.GetApp(ctx, req.Msg.AppId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("app not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	kp, err := ageenv.GenerateKeypair()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	id := uuid.NewString()
	if err := h.store.CreateEnvironment(ctx, dbgen.CreateEnvironmentParams{
		ID:            id,
		AppID:         req.Msg.AppId,
		Name:          req.Msg.Name,
		AgePublicKey:  kp.PublicKey,
		AgePrivateKey: kp.PrivateKey,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	env, err := h.store.GetEnvironment(ctx, dbgen.GetEnvironmentParams{
		AppID: req.Msg.AppId,
		Name:  req.Msg.Name,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&darksidev1.GetEnvironmentResponse{
		Environment: envToProto(env),
		Setup:       buildSetup(app.Name, env.Name, kp.PublicKey, kp.PrivateKey),
	}), nil
}

func envToProto(e dbgen.Environment) *darksidev1.Environment {
	return &darksidev1.Environment{
		Id:            e.ID,
		AppId:         e.AppID,
		Name:          e.Name,
		PublicKey:     e.AgePublicKey,
		PrivateKey:    e.AgePrivateKey,
		CreatedAtUnix: e.CreatedAt.Unix(),
	}
}

func buildSetup(appName, envName, publicKey, privateKey string) *darksidev1.EnvironmentSetup {
	recipientPath := fmt.Sprintf(".darkside/%s.pub", envName)
	envFilePath := fmt.Sprintf("env.%s.age", envName)
	bash := fmt.Sprintf(`# 1. Save the private key locally (kept off the repo).
mkdir -p ~/.darkside/%[1]s
cat > ~/.darkside/%[1]s/%[2]s.txt <<'EOF'
%[3]s
EOF
chmod 600 ~/.darkside/%[1]s/%[2]s.txt

# 2. Commit the public key (recipient) to the repo so anyone can encrypt env files.
mkdir -p .darkside
cat > %[5]s <<'EOF'
%[4]s
EOF

# 3. Write the plaintext env file (do NOT commit env.%[2]s itself), then encrypt:
#    echo "DATABASE_URL=postgres://..." > env.%[2]s
age -R %[5]s -o %[6]s env.%[2]s
rm env.%[2]s

# 4. Commit the public key and the encrypted env file.
git add %[5]s %[6]s
git commit -m "darkside: add %[2]s environment"
`, appName, envName, privateKey, publicKey, recipientPath, envFilePath)

	snippet := fmt.Sprintf("[[environments]]\nname = %q\nenv_file = %q\n", envName, envFilePath)

	return &darksidev1.EnvironmentSetup{
		BashCommands:    bash,
		RecipientPath:   recipientPath,
		EnvFilePath:     envFilePath,
		ManifestSnippet: snippet,
	}
}
