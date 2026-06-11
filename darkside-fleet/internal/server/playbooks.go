package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"connectrpc.com/connect"

	v1 "github.com/singaaka/darkside-fleet/gen/go/fleet/v1"
)

// playbookHandler serves the embedded Ansible playbooks that the runner
// invokes. The dashboard shows them under the Playbooks tab so operators can
// audit exactly what's being applied to each node.
type playbookHandler struct{ s *Server }

func (h *playbookHandler) List(_ context.Context, _ *connect.Request[v1.ListPlaybooksRequest]) (*connect.Response[v1.ListPlaybooksResponse], error) {
	entries, err := fs.ReadDir(h.s.opts.Playbooks, ".")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("read embedded playbooks: %w", err))
	}
	var out []*v1.Playbook
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		pb, err := h.readPlaybook(name)
		if err != nil {
			continue
		}
		// List view returns metadata only (no content) to keep the response light.
		out = append(out, &v1.Playbook{
			Name:        pb.Name,
			SizeBytes:   pb.SizeBytes,
			Description: pb.Description,
		})
	}
	return connect.NewResponse(&v1.ListPlaybooksResponse{Playbooks: out}), nil
}

func (h *playbookHandler) Get(_ context.Context, req *connect.Request[v1.GetPlaybookRequest]) (*connect.Response[v1.Playbook], error) {
	if !isSafePlaybookName(req.Msg.Name) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid playbook name"))
	}
	pb, err := h.readPlaybook(req.Msg.Name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("playbook not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

func (h *playbookHandler) readPlaybook(name string) (*v1.Playbook, error) {
	body, err := fs.ReadFile(h.s.opts.Playbooks, name)
	if err != nil {
		return nil, err
	}
	return &v1.Playbook{
		Name:        name,
		Content:     string(body),
		SizeBytes:   int64(len(body)),
		Description: firstYAMLComment(body),
	}, nil
}

// isSafePlaybookName rejects names with path separators or `..` so we can't
// be tricked into reading arbitrary files. The FS is read-only embedded data,
// but sticking to flat playbook names is what callers expect.
func isSafePlaybookName(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		return false
	}
	return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}

// firstYAMLComment returns the text of the first leading `# comment` line if
// any. Used as a human description for the playbook list view.
func firstYAMLComment(body []byte) string {
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line == "---" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
		return ""
	}
	return ""
}
