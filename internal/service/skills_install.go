package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

const stageProgressInterval = 200 * time.Millisecond

// ReadGithub is M88: parse the address, fetch the repository, unpack it into a
// staging area and list the skills the user can pick (FR-SKM-SRC).
func (s *SkillService) ReadGithub(ctx context.Context, vaultID, input string) contract.SkillStageResult {
	ref, f := skills.ParseGithubInput(input)
	if f != nil {
		return contract.SkillStageResult{Failure: f}
	}
	requestID := progressRequestID(input)
	emit := s.progressEmitter(requestID)
	emit("resolving", 0, true)

	client := s.githubClient(s.app.Version)
	resolved, f := client.Resolve(ctx, ref)
	if f != nil {
		if ctx.Err() != nil {
			return contract.SkillStageResult{Cancelled: true}
		}
		return contract.SkillStageResult{Failure: f}
	}

	store := s.stageStore()
	origin := &contract.SkillOrigin{
		Owner: ref.Owner, Repo: ref.Repo, Ref: resolved.Ref, Path: ref.Path, Commit: resolved.Commit,
	}
	stage, f := store.Create("github", ref.Owner+"/"+ref.Repo, origin)
	if f != nil {
		return contract.SkillStageResult{Failure: f}
	}
	stage.Meta.PathCommit = resolved.PathCommit
	if f := stage.SaveMeta(); f != nil {
		_ = store.Discard(stage.Meta.ID)
		return contract.SkillStageResult{Failure: f}
	}
	fail := func(f *contract.Failure) contract.SkillStageResult {
		_ = store.Discard(stage.Meta.ID)
		return contract.SkillStageResult{Failure: f}
	}

	archivePath := filepath.Join(stage.Dir, "archive.tar.gz")
	archive, err := os.Create(archivePath)
	if err != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法写入暂存区"))
	}
	writer := &progressWriter{writer: archive, emit: func(received int64) { emit("downloading", received, false) }}
	_, f = client.DownloadTarball(ctx, ref, resolved.Commit, writer)
	closeErr := archive.Close()
	if f != nil {
		if ctx.Err() != nil {
			_ = store.Discard(stage.Meta.ID)
			return contract.SkillStageResult{Cancelled: true}
		}
		return fail(f)
	}
	if closeErr != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法写入暂存区"))
	}

	emit("unpacking", 0, true)
	file, err := os.Open(archivePath)
	if err != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法读取暂存区"))
	}
	result, f := skills.ExtractTarGz(file, stage.Root(), skills.ExtractOptions{})
	file.Close()
	_ = os.Remove(archivePath)
	if f != nil {
		return fail(f)
	}

	emit("scanning", 0, true)
	fallback := ref.Repo
	if ref.Path != "" {
		fallback = path.Base(ref.Path)
	}
	stageResult, f := s.findCandidates(ctx, vaultID, stage, fallback, result.Skipped)
	if f != nil {
		return fail(f)
	}
	return stageResult
}

// PickZip is M89: the same pipeline for a local .zip file.
func (s *SkillService) PickZip(ctx context.Context, vaultID string) contract.SkillStageResult {
	paths, err := s.openDialog(ctx, "选择技能 zip 文件", []string{"zip"})
	if err != nil || len(paths) == 0 {
		return contract.SkillStageResult{Cancelled: true}
	}
	stage, f := s.stageStore().Create("zip", filepath.Base(paths[0]), nil)
	if f != nil {
		return contract.SkillStageResult{Failure: f}
	}
	fail := func(f *contract.Failure) contract.SkillStageResult {
		_ = s.stageStore().Discard(stage.Meta.ID)
		return contract.SkillStageResult{Failure: f}
	}
	result, f := skills.ExtractZip(paths[0], stage.Root(), skills.ExtractOptions{})
	if f != nil {
		return fail(f)
	}
	fallback := strings.TrimSuffix(filepath.Base(paths[0]), filepath.Ext(paths[0]))
	stageResult, f := s.findCandidates(ctx, vaultID, stage, fallback, result.Skipped)
	if f != nil {
		return fail(f)
	}
	return stageResult
}

// DiscardStage is M92: the page drops a stage when the user backs out.
func (s *SkillService) DiscardStage(ctx context.Context, stageID string) contract.Failure {
	if f := s.stageStore().Discard(stageID); f != nil {
		return *f
	}
	return contract.Failure{}
}

// findCandidates lists the staged skills and records their hashes for the
// update diff (T09).
func (s *SkillService) findCandidates(ctx context.Context, vaultID string, stage skills.Stage, fallback string, skipped []contract.SkillStageSkip) (contract.SkillStageResult, *contract.Failure) {
	existing := s.existingSkillNames(ctx, vaultID)
	options := skills.CandidateOptions{
		Existing: func(name string) (string, bool) {
			location, ok := existing[strings.ToLower(name)]
			return location, ok
		},
	}
	result, f := skills.FindCandidates(stage.Root(), fallback, options)
	if f != nil {
		return contract.SkillStageResult{}, f
	}
	hashes := map[string]string{}
	for _, candidate := range result.Candidates {
		if candidate.State == "too-large" || candidate.State == "invalid-name" {
			continue
		}
		if hash, err := skills.TreeHash(filepath.Join(stage.Root(), filepath.FromSlash(candidate.ID))); err == nil {
			hashes[candidate.ID] = hash
		}
	}
	if f := stage.SetTreeHashes(hashes); f != nil {
		return contract.SkillStageResult{}, f
	}
	payload := contract.SkillStage{
		StageID:    stage.Meta.ID,
		Kind:       stage.Meta.Kind,
		Origin:     stage.Meta.Origin,
		Label:      stage.Meta.Label,
		Candidates: result.Candidates,
		Skipped:    skipped,
		ExpiresAt:  stage.Meta.ExpiresAt,
	}
	if result.Truncated {
		payload.Skipped = append(payload.Skipped, contract.SkillStageSkip{Path: "", Reason: "capped"})
	}
	return contract.SkillStageResult{Stage: &payload}, nil
}

// existingSkillNames maps lowercased names to their vault path; the read-time
// candidate states and the install-time check use the same view (API §13.3).
func (s *SkillService) existingSkillNames(ctx context.Context, vaultID string) map[string]string {
	existing := map[string]string{}
	if vaultID == "" {
		return existing
	}
	if groups, _, root := s.kbSkills(ctx, vaultID, nil); root != "" {
		for _, group := range groups {
			for _, skill := range group.Skills {
				existing[strings.ToLower(skill.Name)] = skill.Path
			}
		}
	}
	return existing
}

func (s *SkillService) stageStore() *skills.StageStore {
	return skills.NewStageStore(filepath.Join(s.app.DataDir, "skills", "tmp"), s.now)
}

// progressEmitter throttles E13 to one frame per window (16.2).
func (s *SkillService) progressEmitter(requestID string) func(phase string, received int64, force bool) {
	last := time.Time{}
	lastPhase := ""
	return func(phase string, received int64, force bool) {
		now := s.now()
		if !force && phase == lastPhase && now.Sub(last) < stageProgressInterval {
			return
		}
		last, lastPhase = now, phase
		window := s.app.Window()
		if window == nil {
			return
		}
		_ = SkillStageProgress.Emit(window, contract.SkillStageProgressEvent{
			RequestID: requestID,
			Phase:     phase,
			Received:  received,
		})
	}
}

// progressWriter counts the bytes it copies and reports them throttled.
type progressWriter struct {
	writer io.Writer
	emit   func(int64)
	total  int64
	last   time.Time
}

func (w *progressWriter) Write(body []byte) (int, error) {
	written, err := w.writer.Write(body)
	w.total += int64(written)
	now := time.Now()
	if now.Sub(w.last) >= stageProgressInterval {
		w.last = now
		w.emit(w.total)
	}
	return written, err
}

func progressRequestID(input string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(input)))
	return "r-" + hex.EncodeToString(sum[:8])
}
