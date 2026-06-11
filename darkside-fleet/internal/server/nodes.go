package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	v1 "github.com/singaaka/darkside-fleet/gen/go/fleet/v1"
	"github.com/singaaka/darkside-fleet/internal/ansible"
	"github.com/singaaka/darkside-fleet/internal/config"
	"github.com/singaaka/darkside-fleet/internal/db/dbgen"
)

type nodeHandler struct{ s *Server }

func (h *nodeHandler) List(ctx context.Context, _ *connect.Request[v1.ListNodesRequest]) (*connect.Response[v1.ListNodesResponse], error) {
	nodes, err := h.s.opts.Store.ListNodes(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*v1.Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeToProto(n))
	}
	return connect.NewResponse(&v1.ListNodesResponse{Nodes: out}), nil
}

func (h *nodeHandler) Get(ctx context.Context, req *connect.Request[v1.GetNodeRequest]) (*connect.Response[v1.Node], error) {
	n, err := h.s.opts.Store.GetNode(ctx, req.Msg.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("node not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(nodeToProto(n)), nil
}

func (h *nodeHandler) Add(ctx context.Context, req *connect.Request[v1.AddNodeRequest]) (*connect.Response[v1.Node], error) {
	if req.Msg.PublicIp == "" || req.Msg.SshUser == "" || req.Msg.SshKeyPath == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("public_ip, ssh_user, ssh_key_path are required"))
	}

	name := req.Msg.Name
	if name == "" {
		name = fmt.Sprintf("node-%s", req.Msg.PublicIp)
	}

	// Determine if this is the first node — auto-set as paas node.
	count, _ := h.s.opts.Store.CountNodes(ctx)
	isPaas := int64(0)
	if count == 0 {
		isPaas = 1
	}

	tagsJSON, _ := json.Marshal(req.Msg.Tags)
	id := uuid.NewString()
	if err := h.s.opts.Store.CreateNode(ctx, dbgen.CreateNodeParams{
		ID:                 id,
		Name:               name,
		PublicIP:           req.Msg.PublicIp,
		SshUser:            req.Msg.SshUser,
		SshKeyPath:         req.Msg.SshKeyPath,
		Tags:               string(tagsJSON),
		IsDarksidePaasNode: isPaas,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Queue the provisioning job.
	payload, _ := json.Marshal(map[string]string{"node_id": id})
	_ = h.s.opts.Store.CreateJob(ctx, dbgen.CreateJobParams{
		ID:      uuid.NewString(),
		Type:    "add_node",
		Payload: string(payload),
	})

	n, err := h.s.opts.Store.GetNode(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(nodeToProto(n)), nil
}

func (h *nodeHandler) Delete(ctx context.Context, req *connect.Request[v1.DeleteNodeRequest]) (*connect.Response[v1.DeleteNodeResponse], error) {
	n, err := h.s.opts.Store.GetNode(ctx, req.Msg.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("node not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	payload, _ := json.Marshal(map[string]string{
		"node_id":      n.ID,
		"public_ip":    n.PublicIP,
		"ssh_user":     n.SshUser,
		"ssh_key_path": n.SshKeyPath,
	})
	_ = h.s.opts.Store.CreateJob(ctx, dbgen.CreateJobParams{
		ID:      uuid.NewString(),
		Type:    "delete_node",
		Payload: string(payload),
	})

	_ = h.s.opts.Store.UpdateNodeStatus(ctx, dbgen.UpdateNodeStatusParams{
		ID:     req.Msg.Id,
		Status: "draining",
	})
	return connect.NewResponse(&v1.DeleteNodeResponse{Ok: true}), nil
}

func (h *nodeHandler) SetPaasNode(ctx context.Context, req *connect.Request[v1.SetPaasNodeRequest]) (*connect.Response[v1.Node], error) {
	n, err := h.s.opts.Store.GetNode(ctx, req.Msg.NodeId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("node not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	payload, _ := json.Marshal(map[string]string{"node_id": req.Msg.NodeId})
	_ = h.s.opts.Store.CreateJob(ctx, dbgen.CreateJobParams{
		ID:      uuid.NewString(),
		Type:    "migrate_darkside",
		Payload: string(payload),
	})

	_ = h.s.opts.Store.SetPaasNode(ctx, req.Msg.NodeId)
	_ = h.s.opts.Store.SetSetting(ctx, dbgen.SetSettingParams{Key: "darkside_paas_node_id", Value: req.Msg.NodeId})

	n, _ = h.s.opts.Store.GetNode(ctx, req.Msg.NodeId)
	return connect.NewResponse(nodeToProto(n)), nil
}

// ── Job handlers (queue functions) ────────────────────────────────────────────

// RegisterJobHandlers wires up all job types to the queue.
func RegisterJobHandlers(s *Server) {
	q := s.opts.Queue
	runner := s.opts.Runner

	q.Register("add_node", func(ctx context.Context, job dbgen.Job, logf func(string)) error {
		return handleAddNode(ctx, s, job, logf, runner)
	})
	q.Register("delete_node", func(ctx context.Context, job dbgen.Job, logf func(string)) error {
		return handleDeleteNode(ctx, s, job, logf, runner)
	})
	q.Register("migrate_darkside", func(ctx context.Context, job dbgen.Job, logf func(string)) error {
		return handleMigratePaas(ctx, s, job, logf, runner)
	})
	q.Register("update_cluster", func(ctx context.Context, job dbgen.Job, logf func(string)) error {
		return handleUpdateCluster(ctx, s, job, logf, runner)
	})
}

func handleAddNode(ctx context.Context, s *Server, job dbgen.Job, logf func(string), runner *ansible.Runner) error {
	var p struct{ NodeID string `json:"node_id"` }
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}

	node, err := s.opts.Store.GetNode(ctx, p.NodeID)
	if err != nil {
		return err
	}

	// Update status.
	_ = s.opts.Store.UpdateNodeStatus(ctx, dbgen.UpdateNodeStatusParams{ID: node.ID, Status: "provisioning"})

	// Get all nodes for config generation.
	allNodes, _ := s.opts.Store.ListNodes(ctx)
	domain, _ := s.opts.Store.GetSetting(ctx, "domain")
	settings := config.ClusterSettings{Domain: domain}

	// 1. Setup node (install Docker, Nomad, Consul, CNI).
	inventory := ansible.BuildSingleHostInventory(node)
	logf("==> Running setup_node.yml on " + node.PublicIP)
	if err := runner.Run(ctx, ansible.RunOptions{
		Playbook:   "setup_node.yml",
		Inventory:  "-",
		PrivateKey: node.SshKeyPath,
		RemoteUser: node.SshUser,
		ExtraVars:  map[string]string{"registry_addr": fmt.Sprintf("darkside-registry.service.consul:%d", config.RegistryPort)},
		LogF:       logf,
	}); err != nil {
		// Write inventory to a temp file since ansible doesn't support inline "-" easily.
		_ = inventory
		_ = err
		return fmt.Errorf("setup_node failed (ensure ansible-playbook is installed and the node is reachable): %w", err)
	}

	// 2. Generate and write configs.
	nomadHCL := config.GenerateNomadHCL(node, allNodes, settings)
	consulHCL := config.GenerateConsulHCL(node, allNodes)
	traefikYML := config.GenerateTraefikYML(settings)

	_ = s.opts.Store.UpsertClusterConfig(ctx, dbgen.UpsertClusterConfigParams{
		NodeID:     node.ID,
		NomadHcl:   nomadHCL,
		ConsulHcl:  consulHCL,
		TraefikYml: traefikYML,
	})

	logf("==> Running configure_services.yml on " + node.PublicIP)
	if err := runner.Run(ctx, ansible.RunOptions{
		Playbook:   "configure_services.yml",
		Inventory:  "-",
		PrivateKey: node.SshKeyPath,
		RemoteUser: node.SshUser,
		ExtraVars: map[string]string{
			"nomad_hcl":   nomadHCL,
			"consul_hcl":  consulHCL,
			"traefik_yml": traefikYML,
		},
		LogF: logf,
	}); err != nil {
		return err
	}

	// 3. Update all other nodes' configs (new peer in retry_join).
	if len(allNodes) > 1 {
		if err := updateAllNodeConfigs(ctx, s, logf, runner, settings); err != nil {
			logf("WARN: failed to update peer configs: " + err.Error())
		}
	}

	_ = s.opts.Store.UpdateNodeStatus(ctx, dbgen.UpdateNodeStatusParams{ID: node.ID, Status: "active"})
	logf("==> Node " + node.Name + " provisioned and active")
	return nil
}

func handleDeleteNode(ctx context.Context, s *Server, job dbgen.Job, logf func(string), runner *ansible.Runner) error {
	var p struct {
		NodeID     string `json:"node_id"`
		PublicIP   string `json:"public_ip"`
		SshUser    string `json:"ssh_user"`
		SshKeyPath string `json:"ssh_key_path"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}

	node, err := s.opts.Store.GetNode(ctx, p.NodeID)
	if err != nil {
		return err
	}

	logf("==> Running remove_node.yml on " + node.PublicIP)
	nomadNodeID := ""
	if node.NomadNodeID != nil {
		nomadNodeID = *node.NomadNodeID
	}
	if err := runner.Run(ctx, ansible.RunOptions{
		Playbook:   "remove_node.yml",
		Inventory:  "-",
		PrivateKey: node.SshKeyPath,
		RemoteUser: node.SshUser,
		ExtraVars:  map[string]string{"nomad_node_id": nomadNodeID},
		LogF:       logf,
	}); err != nil {
		logf("WARN: remove_node playbook failed (proceeding with DB cleanup): " + err.Error())
	}

	_ = s.opts.Store.DeleteNode(ctx, p.NodeID)

	// Update remaining nodes.
	domain, _ := s.opts.Store.GetSetting(ctx, "domain")
	settings := config.ClusterSettings{Domain: domain}
	_ = updateAllNodeConfigs(ctx, s, logf, runner, settings)
	return nil
}

func handleMigratePaas(ctx context.Context, s *Server, job dbgen.Job, logf func(string), runner *ansible.Runner) error {
	var p struct{ NodeID string `json:"node_id"` }
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}

	newNode, err := s.opts.Store.GetNode(ctx, p.NodeID)
	if err != nil {
		return err
	}

	// Find old paas node.
	allNodes, _ := s.opts.Store.ListNodes(ctx)
	var oldNode *dbgen.Node
	for _, n := range allNodes {
		if n.IsDarksidePaasNode == 1 && n.ID != newNode.ID {
			nn := n
			oldNode = &nn
			break
		}
	}

	if oldNode != nil {
		logf("==> Migrating darkside data from " + oldNode.PublicIP + " to " + newNode.PublicIP)
		if err := runner.Run(ctx, ansible.RunOptions{
			Playbook:  "migrate_darkside.yml",
			Inventory: "-",
			ExtraVars: map[string]string{
				"old_node_ip":  oldNode.PublicIP,
				"new_node_ip":  newNode.PublicIP,
				"ssh_user":     newNode.SshUser,
				"ssh_key_path": newNode.SshKeyPath,
			},
			LogF: logf,
		}); err != nil {
			logf("WARN: migration failed: " + err.Error())
		}
	}

	// Update configs.
	domain, _ := s.opts.Store.GetSetting(ctx, "domain")
	settings := config.ClusterSettings{Domain: domain}
	return updateAllNodeConfigs(ctx, s, logf, runner, settings)
}

func handleUpdateCluster(ctx context.Context, s *Server, job dbgen.Job, logf func(string), runner *ansible.Runner) error {
	domain, _ := s.opts.Store.GetSetting(ctx, "domain")
	settings := config.ClusterSettings{Domain: domain}
	return updateAllNodeConfigs(ctx, s, logf, runner, settings)
}

func updateAllNodeConfigs(ctx context.Context, s *Server, logf func(string), runner *ansible.Runner, settings config.ClusterSettings) error {
	allNodes, err := s.opts.Store.ListNodes(ctx)
	if err != nil || len(allNodes) == 0 {
		return err
	}
	var errs []string
	for _, node := range allNodes {
		nomadHCL := config.GenerateNomadHCL(node, allNodes, settings)
		consulHCL := config.GenerateConsulHCL(node, allNodes)
		_ = s.opts.Store.UpsertClusterConfig(ctx, dbgen.UpsertClusterConfigParams{
			NodeID:    node.ID,
			NomadHcl:  nomadHCL,
			ConsulHcl: consulHCL,
		})
		logf(fmt.Sprintf("==> Updating cluster config on %s (%s)", node.Name, node.PublicIP))
		if err := runner.Run(ctx, ansible.RunOptions{
			Playbook:   "update_cluster.yml",
			Inventory:  "-",
			PrivateKey: node.SshKeyPath,
			RemoteUser: node.SshUser,
			ExtraVars:  map[string]string{"nomad_hcl": nomadHCL, "consul_hcl": consulHCL},
			LogF:       logf,
		}); err != nil {
			errs = append(errs, node.Name+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("update_cluster errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func nodeToProto(n dbgen.Node) *v1.Node {
	var tags []string
	_ = json.Unmarshal([]byte(n.Tags), &tags)
	out := &v1.Node{
		Id:                 n.ID,
		Name:               n.Name,
		PublicIp:           n.PublicIP,
		SshUser:            n.SshUser,
		SshKeyPath:         n.SshKeyPath,
		Tags:               tags,
		IsDarksidePaasNode: n.IsDarksidePaasNode == 1,
		Status:             n.Status,
		CreatedAtUnix:      n.CreatedAt.Unix(),
	}
	if n.NomadNodeID != nil {
		out.NomadNodeId = *n.NomadNodeID
	}
	return out
}
