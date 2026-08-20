package agentassets

import (
	"context"
	"fmt"
	"sync"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

type key struct {
	id      string
	version int
}

// Registry 是进程内只读的内置资源注册表；构建后不暴露可变引用。
type Registry struct {
	mu         sync.RWMutex
	workflows  map[key]ports.WorkflowAsset
	prompts    map[key]ports.PromptAsset
	schemas    map[key]ports.SchemaAsset
	assemblers map[key]ports.AssemblerAsset
}

func New(
	workflows []ports.WorkflowAsset,
	prompts []ports.PromptAsset,
	schemas []ports.SchemaAsset,
	assemblers []ports.AssemblerAsset,
) (*Registry, error) {
	registry := &Registry{
		workflows: make(map[key]ports.WorkflowAsset, len(workflows)), prompts: make(map[key]ports.PromptAsset, len(prompts)),
		schemas: make(map[key]ports.SchemaAsset, len(schemas)), assemblers: make(map[key]ports.AssemblerAsset, len(assemblers)),
	}
	for _, asset := range workflows {
		if asset.Binding.ID != asset.Definition.ID || asset.Binding.Version != asset.Definition.Version ||
			asset.Binding.Hash != asset.Definition.DefinitionHash || asset.Binding.Hash != asset.Definition.ContentHash() {
			return nil, fmt.Errorf("workflow asset hash mismatch: %s@%d", asset.Binding.ID, asset.Binding.Version)
		}
		if err := asset.Definition.Validate(); err != nil {
			return nil, err
		}
		if err := put(registry.workflows, asset.Binding, copyWorkflow(asset)); err != nil {
			return nil, err
		}
	}
	for _, asset := range prompts {
		if asset.Binding.Hash != agentdomain.SHA256Hex([]byte(asset.Template)) {
			return nil, fmt.Errorf("prompt asset hash mismatch: %s@%d", asset.Binding.ID, asset.Binding.Version)
		}
		if err := put(registry.prompts, asset.Binding, copyPrompt(asset)); err != nil {
			return nil, err
		}
	}
	for _, asset := range schemas {
		if asset.Binding.Hash != agentdomain.SHA256Hex(asset.JSON) {
			return nil, fmt.Errorf("schema asset hash mismatch: %s@%d", asset.Binding.ID, asset.Binding.Version)
		}
		if err := put(registry.schemas, asset.Binding, copySchema(asset)); err != nil {
			return nil, err
		}
	}
	for _, asset := range assemblers {
		if asset.Binding.Hash != agentdomain.SHA256Hex(asset.JSON) {
			return nil, fmt.Errorf("assembler asset hash mismatch: %s@%d", asset.Binding.ID, asset.Binding.Version)
		}
		if err := put(registry.assemblers, asset.Binding, copyAssembler(asset)); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (registry *Registry) Workflow(_ context.Context, id string, version int) (ports.WorkflowAsset, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	asset, ok := registry.workflows[key{id, version}]
	if !ok {
		return ports.WorkflowAsset{}, fmt.Errorf("%w: workflow %s@%d", ports.ErrAgentAssetNotFound, id, version)
	}
	return copyWorkflow(asset), nil
}

func (registry *Registry) Prompt(_ context.Context, id string, version int) (ports.PromptAsset, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	asset, ok := registry.prompts[key{id, version}]
	if !ok {
		return ports.PromptAsset{}, fmt.Errorf("%w: prompt %s@%d", ports.ErrAgentAssetNotFound, id, version)
	}
	return copyPrompt(asset), nil
}

func (registry *Registry) Schema(_ context.Context, id string, version int) (ports.SchemaAsset, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	asset, ok := registry.schemas[key{id, version}]
	if !ok {
		return ports.SchemaAsset{}, fmt.Errorf("%w: schema %s@%d", ports.ErrAgentAssetNotFound, id, version)
	}
	return copySchema(asset), nil
}

func (registry *Registry) Assembler(_ context.Context, id string, version int) (ports.AssemblerAsset, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	asset, ok := registry.assemblers[key{id, version}]
	if !ok {
		return ports.AssemblerAsset{}, fmt.Errorf("%w: assembler %s@%d", ports.ErrAgentAssetNotFound, id, version)
	}
	return copyAssembler(asset), nil
}

func put[T any](target map[key]T, binding agentdomain.VersionBinding, value T) error {
	if err := binding.Validate(); err != nil {
		return err
	}
	assetKey := key{binding.ID, binding.Version}
	if _, exists := target[assetKey]; exists {
		return fmt.Errorf("duplicate agent asset %s@%d", binding.ID, binding.Version)
	}
	target[assetKey] = value
	return nil
}

func copyWorkflow(asset ports.WorkflowAsset) ports.WorkflowAsset {
	asset.Definition.Nodes = append([]agentdomain.WorkflowNode(nil), asset.Definition.Nodes...)
	for index := range asset.Definition.Nodes {
		asset.Definition.Nodes[index].InputSelectors = append([]string(nil), asset.Definition.Nodes[index].InputSelectors...)
	}
	asset.Definition.Edges = append([]agentdomain.WorkflowEdge(nil), asset.Definition.Edges...)
	return asset
}

func copyPrompt(asset ports.PromptAsset) ports.PromptAsset {
	asset.Variables = append([]string(nil), asset.Variables...)
	return asset
}

func copySchema(asset ports.SchemaAsset) ports.SchemaAsset {
	asset.JSON = append([]byte(nil), asset.JSON...)
	return asset
}
func copyAssembler(asset ports.AssemblerAsset) ports.AssemblerAsset {
	asset.JSON = append([]byte(nil), asset.JSON...)
	return asset
}

// BindingForText 为内置文本资源创建内容寻址的版本绑定。
func BindingForText(id string, version int, content []byte) agentdomain.VersionBinding {
	return agentdomain.VersionBinding{ID: id, Version: version, Hash: agentdomain.SHA256Hex(content)}
}
