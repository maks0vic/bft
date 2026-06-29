package config

import (
	"fmt"

	"bft/internal/model"
)

type RuntimeCluster struct {
	Nodes []model.NodeConfig
}

func BuildRuntimeCluster(req model.StartRequest, basePort int) (RuntimeCluster, error) {
	if err := validateStartRequest(req); err != nil {
		return RuntimeCluster{}, err
	}

	nodes := make([]model.NodeConfig, 0, req.NodeCount)
	byzantineStart := req.NodeCount - req.ByzantineCount + 1

	for i := 1; i <= req.NodeCount; i++ {
		cfg := model.NodeConfig{
			ID:      fmt.Sprintf("node%d", i),
			Address: fmt.Sprintf("localhost:%d", basePort+i-1),
			Leader:  i == 1,
		}
		if i >= byzantineStart {
			cfg.Byzantine = true
			cfg.Behavior = req.ByzantineBehavior
		}
		nodes = append(nodes, cfg)
	}

	for i := range nodes {
		peers := make([]model.Peer, 0, len(nodes)-1)
		for j := range nodes {
			if i == j {
				continue
			}
			peers = append(peers, model.Peer{
				ID:        nodes[j].ID,
				Address:   nodes[j].Address,
				Byzantine: nodes[j].Byzantine,
			})
		}
		nodes[i].Peers = peers
	}

	return RuntimeCluster{Nodes: nodes}, nil
}

func validateStartRequest(req model.StartRequest) error {
	if req.Value == "" {
		return fmt.Errorf("value is required")
	}
	if req.NodeCount < 4 {
		return fmt.Errorf("nodeCount must be at least 4")
	}
	if req.ByzantineCount < 1 {
		return fmt.Errorf("byzantineCount must be at least 1")
	}
	if req.NodeCount < 3*req.ByzantineCount+1 {
		return fmt.Errorf("nodeCount must satisfy 3f + 1 for byzantineCount=%d", req.ByzantineCount)
	}
	if req.NodeCount-req.ByzantineCount < 1 {
		return fmt.Errorf("at least one honest node is required")
	}
	switch req.ByzantineBehavior {
	case model.BehaviorSilent,
		model.BehaviorConflictingValue,
		model.BehaviorInvalidLeaderProposal,
		model.BehaviorStaleViewSpam,
		model.BehaviorMalformedCertificate,
		model.BehaviorEquivocatingViewChange:
	default:
		return fmt.Errorf("unsupported byzantine behavior %q", req.ByzantineBehavior)
	}
	return nil
}
