package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/mutation"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// MutationService turns user actions into confirmed change plans (API.md M20–
// M23, M29). Planning never writes to the vault.
type MutationService struct{ app *App }

// NewMutationService binds one app.
func NewMutationService(app *App) *MutationService { return &MutationService{app: app} }

// Execute applies a confirmed plan (M30). Edits go through the same
// revision-checked writer as a normal save; trash uses the system recycle bin.
func (s *MutationService) Execute(ctx context.Context, plan contract.MutationPlan) contract.MutationReport {
	report := contract.MutationReport{PlanID: plan.ID, Status: "failed",
		Completed: []contract.ActionRecord{}, Failed: []contract.ActionRecord{},
		Remaining: []contract.ActionRecord{}, TouchedDirectories: []string{}}
	if f := validateID(plan.VaultID); f != nil {
		report.Failure = f
		return report
	}
	root, f := s.app.Vaults.Root(plan.VaultID)
	if f != nil {
		report.Failure = f
		return report
	}
	executor := mutation.Executor{
		Root:    root,
		VaultID: plan.VaultID,
		Manager: mutation.TxManager{Root: root},
		Trash: func(abs string) error {
			return mygo.Shell.TrashItem(abs)
		},
		WriteText: func(rel string, expectedRevision int64, text string) (string, *contract.Failure) {
			outcome, wf := vault.Write(root, rel, expectedRevision, text)
			if wf != nil {
				return "", wf
			}
			if outcome.Status == "conflict" {
				return "", contract.Fail(contract.CodeRevisionMismatch, "目标已被外部修改")
			}
			return outcome.Hash, nil
		},
	}
	return executor.Execute(plan)
}

// Plan validates a change plan (M29).
func (s *MutationService) Plan(ctx context.Context, req contract.PlanRequest) contract.PlanResult {
	return s.build(req)
}

// Rename plans a move that only changes the entry name (M20).
func (s *MutationService) Rename(ctx context.Context, vaultID, path, newName string) contract.PlanResult {
	return s.build(contract.PlanRequest{
		VaultID: vaultID,
		Origin:  "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutMove, Source: path,
			Target: joinRel(parentOf(path), newName), Reason: "用户重命名",
		}},
	})
}

// CreateFile plans a new file inside dirPath (M21).
func (s *MutationService) CreateFile(ctx context.Context, vaultID, dirPath, name string) contract.PlanResult {
	return s.build(contract.PlanRequest{
		VaultID: vaultID,
		Origin:  "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutCreate,
			Target: joinRel(dirPath, name), Reason: "用户新建文件",
		}},
	})
}

// CreateDir plans a new directory inside dirPath (M22).
func (s *MutationService) CreateDir(ctx context.Context, vaultID, dirPath, name string) contract.PlanResult {
	return s.build(contract.PlanRequest{
		VaultID: vaultID,
		Origin:  "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutCreateDirectory,
			Target: joinRel(dirPath, name), Reason: "用户新建目录",
		}},
	})
}

// PlanMove plans moving several entries into targetDir (M23).
func (s *MutationService) PlanMove(ctx context.Context, vaultID string, sources []string, targetDir string) contract.PlanResult {
	actions := make([]contract.MutationAction, 0, len(sources))
	for index, source := range sources {
		actions = append(actions, contract.MutationAction{
			ID: "a" + strconv.Itoa(index+1), Kind: contract.MutMove, Source: source,
			Target: joinRel(targetDir, baseName(source)), Reason: "用户移动",
		})
	}
	return s.build(contract.PlanRequest{VaultID: vaultID, Origin: "user", Actions: actions})
}

func (s *MutationService) build(req contract.PlanRequest) contract.PlanResult {
	if f := validateID(req.VaultID); f != nil {
		return contract.PlanResult{Failure: f}
	}
	root, f := s.app.Vaults.Root(req.VaultID)
	if f != nil {
		return contract.PlanResult{Failure: f}
	}
	builder := mutation.Builder{Root: root}
	plan, failure := builder.Plan(req)
	if failure != nil {
		return contract.PlanResult{Failure: failure}
	}
	return contract.PlanResult{Plan: &plan}
}

func parentOf(rel string) string {
	if at := strings.LastIndex(rel, "/"); at >= 0 {
		return rel[:at]
	}
	return ""
}

func baseName(rel string) string {
	if at := strings.LastIndex(rel, "/"); at >= 0 {
		return rel[at+1:]
	}
	return rel
}

func joinRel(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}
