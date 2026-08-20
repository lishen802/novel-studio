package ports

import (
	"context"
	"errors"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var ErrAgentAssetNotFound = errors.New("agent asset not found")

type PromptAsset struct {
	Binding   agentdomain.VersionBinding
	Template  string
	Variables []string
}

type SchemaAsset struct {
	Binding agentdomain.VersionBinding
	JSON    []byte
}

type AssemblerAsset struct {
	Binding agentdomain.VersionBinding
	JSON    []byte
}

type WorkflowAsset struct {
	Binding    agentdomain.VersionBinding
	Definition agentdomain.WorkflowDefinition
}

// 以下注册表只接受精确版本，调用方不得使用“latest”隐式漂移。
type WorkflowRegistry interface {
	Workflow(ctx context.Context, id string, version int) (WorkflowAsset, error)
}

type PromptRegistry interface {
	Prompt(ctx context.Context, id string, version int) (PromptAsset, error)
}

type SchemaRegistry interface {
	Schema(ctx context.Context, id string, version int) (SchemaAsset, error)
}

type AssemblerRegistry interface {
	Assembler(ctx context.Context, id string, version int) (AssemblerAsset, error)
}
