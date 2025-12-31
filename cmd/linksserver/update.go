package main

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

const goInstallTarget = "github.com/tomek7667/links/cmd/linksserver@latest"

type updateState struct {
	CreatedAt time.Time `json:"createdAt"`

	TargetPath string `json:"targetPath"`
	BackupPath string `json:"backupPath"`
	StagePath  string `json:"stagePath"`
	StageLabel string `json:"stageLabel,omitempty"`

	DBPath       string `json:"dbPath,omitempty"`
	DBBackupPath string `json:"dbBackupPath,omitempty"`

	FromVersion  string `json:"fromVersion"`
	FromRevision string `json:"fromRevision,omitempty"`
	FromModified bool   `json:"fromModified,omitempty"`

	ToVersion  string `json:"toVersion"`
	ToRevision string `json:"toRevision,omitempty"`
	ToModified bool   `json:"toModified,omitempty"`
}

type buildMeta struct {
	version  string
	revision string
	modified bool
}

func cmdUpdate() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Install the latest version (keeps a backup until complete-update)",
		Action: func(c *cli.Context) error {
			return runUpdate(c.Context)
		},
	}
}

func cmdCompleteUpdate() *cli.Command {
	return &cli.Command{
		Name:  "complete-update",
		Usage: "Finalize a previous update by removing the backup and temporary files",
		Action: func(c *cli.Context) error {
			return runCompleteUpdate()
		},
	}
}

func runUpdate(ctx context.Context) error {
	exePath, err := currentExecutablePath()
	if err != nil {
		return err
	}
	statePath := updateStatePath(exePath)
	if _, err := os.Stat(statePath); err == nil {
		return fmt.Errorf("update already pending; run %q to clean up (%s)", fmt.Sprintf("%s complete-update", filepath.Base(exePath)), statePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat update state file: %w", err)
	}

	currentBI, _ := debug.ReadBuildInfo()
	currentMeta := metaFromBuildInfo(currentBI)
	fmt.Printf("current version: %s\n", printableVersion(currentMeta))

	now := time.Now().UTC()

	tmpDir, err := os.MkdirTemp("", "linksserver-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	goExe, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found in PATH; cannot self-update (try: `go install %s`)", goInstallTarget)
	}

	fmt.Printf("fetching latest via `go install %s`...\n", goInstallTarget)
	cmd := exec.CommandContext(ctx, goExe, "install", goInstallTarget)
	cmd.Env = append(os.Environ(), "GOBIN="+tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("failed to run `go install %s`: %w", goInstallTarget, err)
		}
		return fmt.Errorf("failed to run `go install %s`: %w\n\n%s", goInstallTarget, err, msg)
	}

	latestBinPath, err := installedBinaryPath(tmpDir)
	if err != nil {
		return err
	}
	latestBI, err := buildinfo.ReadFile(latestBinPath)
	if err != nil {
		return fmt.Errorf("failed to read build info from %s: %w", latestBinPath, err)
	}
	latestMeta := metaFromBuildInfo(latestBI)
	fmt.Printf("latest available: %s\n", printableVersion(latestMeta))

	if isSameBuild(currentMeta, latestMeta) {
		if currentMeta.version != "" {
			fmt.Printf("already latest (%s)\n", currentMeta.version)
		} else {
			fmt.Printf("already latest\n")
		}
		return nil
	}

	stageLabel := versionLabel(latestMeta, now)
	stagePath := stageBinaryPath(exePath, stageLabel, now)
	backupPath := backupBinaryPath(exePath, now)

	mode, err := executableFileMode(exePath)
	if err != nil {
		return err
	}

	fmt.Println("creating backup and staging new binary...")
	if err := copyFile(exePath, backupPath, mode); err != nil {
		return fmt.Errorf("failed to create backup at %s: %w", backupPath, err)
	}
	if err := copyFile(latestBinPath, stagePath, mode); err != nil {
		return fmt.Errorf("failed to stage updated binary at %s: %w", stagePath, err)
	}

	dbPath, dbBackupPath, err := backupDBIfPresent(exePath, now)
	if err != nil {
		return fmt.Errorf("failed to create database backup: %w", err)
	}

	state := updateState{
		CreatedAt:    now,
		TargetPath:   exePath,
		BackupPath:   backupPath,
		StagePath:    stagePath,
		StageLabel:   stageLabel,
		DBPath:       dbPath,
		DBBackupPath: dbBackupPath,
		FromVersion:  currentMeta.version, FromRevision: currentMeta.revision, FromModified: currentMeta.modified,
		ToVersion: latestMeta.version, ToRevision: latestMeta.revision, ToModified: latestMeta.modified,
	}
	if err := writeJSONFileAtomic(statePath, state, 0o644); err != nil {
		return fmt.Errorf("failed to write update state file %s: %w", statePath, err)
	}

	fmt.Printf("staged new binary at: %s\n", stagePath)
	fmt.Printf("binary backup: %s\n", backupPath)
	if dbBackupPath != "" {
		fmt.Printf("database backed up: %s (from %s)\n", dbBackupPath, dbPath)
	} else {
		fmt.Println("no database found to back up (expected links.db.json next to the binary or cwd)")
	}
	fmt.Printf("run the staged binary to test: %s\n", stagePath)
	fmt.Printf("when satisfied, finalize with: %s complete-update\n", filepath.Base(exePath))
	return nil
}

func runCompleteUpdate() error {
	exePath, err := currentExecutablePath()
	if err != nil {
		return err
	}
	statePath := updateStatePath(exePath)
	b, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("no pending update")
			return nil
		}
		return fmt.Errorf("failed to read update state file %s: %w", statePath, err)
	}

	var state updateState
	if err := json.Unmarshal(b, &state); err != nil {
		return fmt.Errorf("failed to parse update state file %s: %w", statePath, err)
	}

	curBI, _ := debug.ReadBuildInfo()
	curMeta := metaFromBuildInfo(curBI)

	stageInfo, err := buildinfo.ReadFile(state.StagePath)
	if err != nil {
		return fmt.Errorf("failed to read build info from staged binary %s: %w", state.StagePath, err)
	}
	stageMeta := metaFromBuildInfo(stageInfo)
	toMeta := buildMeta{version: state.ToVersion, revision: state.ToRevision, modified: state.ToModified}
	if !isSameBuild(stageMeta, toMeta) {
		return fmt.Errorf("staged binary at %s does not match expected update (expected %s, got %s)", state.StagePath, printableVersion(toMeta), printableVersion(stageMeta))
	}
	if !isSameBuild(curMeta, toMeta) && !isSameBuild(curMeta, buildMeta{version: state.FromVersion, revision: state.FromRevision, modified: state.FromModified}) {
		fmt.Printf("warning: completing update while running %s (promoting %s)\n", printableVersion(curMeta), printableVersion(stageMeta))
	}

	if _, err := os.Stat(state.StagePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("staged binary missing at %s; rerun update", state.StagePath)
		}
		return fmt.Errorf("failed to access staged binary %s: %w", state.StagePath, err)
	}

	if runtime.GOOS == "windows" {
		if err := spawnWindowsFinalizeScript(os.Getpid(), statePath, state); err != nil {
			return err
		}
		fmt.Println("finalizing update in the background; this process can exit now.")
	} else {
		fmt.Println("promoting staged binary...")
		if err := promoteStagedBinary(state); err != nil {
			return err
		}
		if err := cleanupUpdateArtifacts(statePath, state); err != nil {
			return err
		}
		fmt.Println("update completed; backups cleaned up")
	}

	return nil
}

func printableVersion(m buildMeta) string {
	if m.version != "" && m.version != "(devel)" {
		return m.version
	}
	if m.revision != "" {
		if m.modified {
			return m.revision + " (modified)"
		}
		return m.revision
	}
	if m.version != "" {
		return m.version
	}
	return "unknown"
}

func currentExecutablePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current executable path: %w", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("failed to make executable path absolute: %w", err)
	}
	return exePath, nil
}

func updateStatePath(exePath string) string {
	return exePath + ".update.json"
}

func executableFileMode(exePath string) (os.FileMode, error) {
	fi, err := os.Stat(exePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat %s: %w", exePath, err)
	}
	return fi.Mode(), nil
}

func versionLabel(m buildMeta, now time.Time) string {
	if m.version != "" && m.version != "(devel)" {
		return m.version
	}
	if m.revision != "" {
		return m.revision
	}
	return now.Format("20060102T150405Z")
}

func stageBinaryPath(exePath, label string, now time.Time) string {
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	safeLabel := sanitizeForFilename(label)
	stage := filepath.Join(dir, fmt.Sprintf("%s-%s%s", name, safeLabel, ext))
	if _, err := os.Stat(stage); err == nil {
		stage = filepath.Join(dir, fmt.Sprintf("%s-%s-%s%s", name, safeLabel, now.Format("20060102T150405Z"), ext))
	}
	return stage
}

func backupBinaryPath(exePath string, now time.Time) string {
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	ts := now.Format("20060102T150405Z")
	return filepath.Join(dir, fmt.Sprintf("%s.backup-%s%s", name, ts, ext))
}

func backupDBIfPresent(exePath string, now time.Time) (dbPath, backupPath string, err error) {
	candidates := []string{}
	exeDir := filepath.Dir(exePath)
	candidates = append(candidates, filepath.Join(exeDir, "links.db.json"))
	if wd, wdErr := os.Getwd(); wdErr == nil {
		if wd != exeDir {
			candidates = append(candidates, filepath.Join(wd, "links.db.json"))
		}
	}

	seen := map[string]struct{}{}
	for _, c := range candidates {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		info, statErr := os.Stat(c)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return "", "", fmt.Errorf("failed to stat %s: %w", c, statErr)
		}
		if info.IsDir() {
			continue
		}
		dbPath = c
		mode := info.Mode()
		backupPath = fmt.Sprintf("%s.bak-%s", dbPath, now.Format("20060102T150405Z"))
		if copyErr := copyFile(dbPath, backupPath, mode); copyErr != nil {
			return "", "", copyErr
		}
		return dbPath, backupPath, nil
	}
	return "", "", nil
}

func installedBinaryPath(dir string) (string, error) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	p := filepath.Join(dir, "linksserver"+ext)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read temp bin dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			continue
		}
		return filepath.Join(dir, e.Name()), nil
	}
	return "", fmt.Errorf("could not find installed binary in %s", dir)
}

func metaFromBuildInfo(bi *debug.BuildInfo) buildMeta {
	if bi == nil {
		return buildMeta{}
	}
	m := buildMeta{version: bi.Main.Version}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			m.revision = s.Value
		case "vcs.modified":
			m.modified = (s.Value == "true")
		}
	}
	return m
}

func isSameBuild(a, b buildMeta) bool {
	if a.version != "" && b.version != "" && a.version == b.version && a.version != "(devel)" {
		return true
	}
	if a.revision != "" && b.revision != "" && a.revision == b.revision && !a.modified && !b.modified {
		return true
	}
	return a.version == b.version && a.revision == b.revision && a.modified == b.modified
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	_ = os.Chmod(dst, mode)
	return nil
}

func replaceFile(srcPath, dstPath string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("cannot replace running executable on Windows; update will finish in background")
	}
	if err := os.Remove(dstPath); err != nil {
		return err
	}
	return os.Rename(srcPath, dstPath)
}

func writeJSONFileAtomic(path string, v any, mode os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if mode != 0 {
		_ = os.Chmod(tmpName, mode)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func promoteStagedBinary(state updateState) error {
	if err := os.Remove(state.TargetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove old binary %s: %w", state.TargetPath, err)
	}
	if err := os.Rename(state.StagePath, state.TargetPath); err != nil {
		return fmt.Errorf("failed to promote %s to %s: %w", state.StagePath, state.TargetPath, err)
	}
	return nil
}

func cleanupUpdateArtifacts(statePath string, state updateState) error {
	if state.BackupPath != "" {
		if err := os.Remove(state.BackupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove backup %s: %w", state.BackupPath, err)
		}
	}
	if state.DBBackupPath != "" {
		if err := os.Remove(state.DBBackupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove db backup %s: %w", state.DBBackupPath, err)
		}
	}
	if err := os.Remove(statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove update state file %s: %w", statePath, err)
	}
	return nil
}

func spawnWindowsFinalizeScript(pid int, statePath string, state updateState) error {
	script, err := os.CreateTemp("", "linksserver-swap-*.cmd")
	if err != nil {
		return fmt.Errorf("failed to create swap helper script: %w", err)
	}
	scriptPath := script.Name()

	contents := fmt.Sprintf(`@echo off
setlocal
set "PID=%d"
set "TARGET=%s"
set "STAGE=%s"
set "BACKUP=%s"
set "DBBACKUP=%s"
set "STATE=%s"

:wait
tasklist /FI "PID eq %%PID%%" 2>nul | find "%%PID%%" >nul
if %%ERRORLEVEL%%==0 (
  timeout /T 1 /NOBREAK >nul
  goto wait
)

del /F /Q "%%TARGET%%" >nul 2>nul
move /Y "%%STAGE%%" "%%TARGET%%" >nul 2>nul
if errorlevel 1 goto fail

if not "%%BACKUP%%"=="" del /F /Q "%%BACKUP%%" >nul 2>nul
if not "%%DBBACKUP%%"=="" del /F /Q "%%DBBACKUP%%" >nul 2>nul
if not "%%STATE%%"=="" del /F /Q "%%STATE%%" >nul 2>nul
goto cleanup

:fail
echo linksserver complete-update: failed to replace "%%TARGET%%" from "%%STAGE%%"
:cleanup
del "%%~f0" >nul 2>nul
exit /B 0
`, pid, escapeForCmd(state.TargetPath), escapeForCmd(state.StagePath), escapeForCmd(state.BackupPath), escapeForCmd(state.DBBackupPath), escapeForCmd(statePath))

	if _, err := script.WriteString(contents); err != nil {
		script.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to write swap helper script: %w", err)
	}
	if err := script.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to close swap helper script: %w", err)
	}

	c := exec.Command("cmd.exe", "/C", scriptPath)
	c.Stdout = nil
	c.Stderr = nil
	if err := c.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to start swap helper: %w", err)
	}
	return nil
}

func escapeForCmd(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

func sanitizeForFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}
