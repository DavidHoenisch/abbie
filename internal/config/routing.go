package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// RoutingList is an ordered list of routing rules. In YAML you may use either a
// single rule object (legacy) or a list; list order is the order of evaluation.
type RoutingList []RoutingRule

// UnmarshalYAML accepts a mapping (one rule) or a sequence (ordered rules).
func (rl *RoutingList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var rules []RoutingRule
		if err := node.Decode(&rules); err != nil {
			return err
		}
		if len(rules) == 0 {
			return fmt.Errorf("routing: at least one rule is required")
		}
		*rl = rules
		return nil
	case yaml.MappingNode:
		var single RoutingRule
		if err := node.Decode(&single); err != nil {
			return err
		}
		*rl = RoutingList{single}
		return nil
	default:
		return fmt.Errorf("routing: must be a list of rules or one rule object")
	}
}

func validateRouting(cfg *Config) error {
	rl := cfg.Routing
	if len(rl) == 0 {
		return fmt.Errorf("routing: at least one rule is required")
	}
	backendNames := make(map[string]struct{}, len(cfg.Backends))
	for _, b := range cfg.Backends {
		if b.Name != "" {
			backendNames[b.Name] = struct{}{}
		}
	}

	for i, rule := range rl {
		switch rule.Strategy {
		case RoundRobin, QueryParam, Header, Cookie, Static:
		default:
			return fmt.Errorf("routing[%d]: unknown strategy %q", i, rule.Strategy)
		}
		if rule.Strategy != RoundRobin {
			continue
		}
		for _, t := range rule.Targets {
			if t == "" {
				return fmt.Errorf("routing[%d]: round-robin targets must not contain empty names", i)
			}
			if _, ok := backendNames[t]; !ok {
				return fmt.Errorf("routing[%d]: round-robin targets unknown backend %q", i, t)
			}
		}
	}
	return nil
}
