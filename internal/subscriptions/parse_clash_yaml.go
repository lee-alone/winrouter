package subscriptions

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"winrouter/internal/nodes"
)

const (
	maxYAMLDepth   = 10
	maxYAMLNodes   = 5000
	maxYAMLAliases = 20
)

func validateYAMLNodeTree(node *yaml.Node, currentDepth int, nodeCount, aliasCount *int) error {
	if node == nil {
		return nil
	}
	*nodeCount++
	if *nodeCount > maxYAMLNodes {
		return fmt.Errorf("YAML node count exceeds safety limit of %d", maxYAMLNodes)
	}
	if currentDepth > maxYAMLDepth {
		return fmt.Errorf("YAML structure exceeds maximum depth limit of %d", maxYAMLDepth)
	}
	if node.Kind == yaml.AliasNode {
		*aliasCount++
		if *aliasCount > maxYAMLAliases {
			return fmt.Errorf("YAML alias count exceeds safety limit of %d", maxYAMLAliases)
		}
	}
	for _, child := range node.Content {
		if err := validateYAMLNodeTree(child, currentDepth+1, nodeCount, aliasCount); err != nil {
			return err
		}
	}
	return nil
}

var allowedClashProxyKeys = map[string]bool{
	"name":             true,
	"type":             true,
	"server":           true,
	"port":             true,
	"username":         true,
	"password":         true,
	"cipher":           true,
	"method":           true,
	"uuid":             true,
	"alterid":          true,
	"alter-id":         true,
	"alter_id":         true,
	"flow":             true,
	"tls":              true,
	"servername":       true,
	"sni":              true,
	"skip-cert-verify": true,
	"alpn":             true,
	"network":          true,
	"ws-opts":          true,
	"ws-path":          true,
	"ws-headers":       true,
	"udp":              true,
}

func findProxiesSeq(root *yaml.Node) (*yaml.Node, error) {
	if root == nil {
		return nil, errors.New("nil YAML root")
	}
	var docMap *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		docMap = root.Content[0]
	} else if root.Kind == yaml.MappingNode {
		docMap = root
	}
	if docMap == nil || docMap.Kind != yaml.MappingNode {
		return nil, errors.New("root of Clash YAML must be a mapping")
	}

	for i := 0; i < len(docMap.Content); i += 2 {
		keyNode := docMap.Content[i]
		key := strings.ToLower(strings.TrimSpace(keyNode.Value))
		if key == "proxies" {
			proxiesSeq := docMap.Content[i+1]
			for proxiesSeq.Kind == yaml.AliasNode {
				proxiesSeq = proxiesSeq.Alias
			}
			if proxiesSeq.Kind != yaml.SequenceNode {
				return nil, errors.New("proxies section must be a list")
			}
			return proxiesSeq, nil
		}
	}
	return nil, errors.New("YAML contains no proxies section")
}

func validateProxyNodeAST(proxyNode *yaml.Node) error {
	actual := proxyNode
	for actual.Kind == yaml.AliasNode {
		actual = actual.Alias
	}
	if actual.Kind != yaml.MappingNode {
		return errors.New("proxy entry must be a YAML mapping")
	}
	for j := 0; j < len(actual.Content); j += 2 {
		kNode := actual.Content[j]
		vNode := actual.Content[j+1]
		key := strings.ToLower(strings.TrimSpace(kNode.Value))
		if !allowedClashProxyKeys[key] {
			return fmt.Errorf("unsupported or unknown proxy field %q", kNode.Value)
		}
		if key == "ws-opts" && vNode != nil {
			wsActual := vNode
			for wsActual.Kind == yaml.AliasNode {
				wsActual = wsActual.Alias
			}
			if wsActual.Kind == yaml.MappingNode {
				for w := 0; w < len(wsActual.Content); w += 2 {
					wKey := strings.ToLower(strings.TrimSpace(wsActual.Content[w].Value))
					if wKey != "path" && wKey != "headers" {
						return fmt.Errorf("unsupported ws-opts field %q (only path and headers are supported)", wsActual.Content[w].Value)
					}
				}
			}
		}
	}
	return nil
}

func parseClashYAML(data []byte) ([]nodes.Input, error) {
	if len(data) > 10*1024*1024 {
		return nil, errors.New("YAML configuration exceeds 10MB limit")
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid YAML syntax: %w", err)
	}

	nodeCount := 0
	aliasCount := 0
	if err := validateYAMLNodeTree(&root, 1, &nodeCount, &aliasCount); err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	proxiesSeq, err := findProxiesSeq(&root)
	if err != nil {
		return nil, err
	}
	if len(proxiesSeq.Content) == 0 {
		return nil, errors.New("YAML contains no proxies")
	}
	if len(proxiesSeq.Content) > 500 {
		return nil, errors.New("subscription must contain 1 to 500 nodes")
	}

	result := make([]nodes.Input, 0, len(proxiesSeq.Content))
	var lastErr error
	for idx, proxyNode := range proxiesSeq.Content {
		if err := validateProxyNodeAST(proxyNode); err != nil {
			lastErr = fmt.Errorf("proxy node %d: %w", idx+1, err)
			continue
		}
		var p clashProxy
		if err := proxyNode.Decode(&p); err != nil {
			lastErr = fmt.Errorf("proxy node %d: %w", idx+1, err)
			continue
		}
		item, err := mapClashProxy(p)
		if err != nil {
			lastErr = fmt.Errorf("proxy node %d (%q): %w", idx+1, p.Name, err)
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("all Clash proxies failed to parse: %w", lastErr)
		}
		return nil, errors.New("no valid proxies found in Clash YAML")
	}
	return result, nil
}
